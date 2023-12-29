package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

// WalkedFile a type of struct
type WalkedFile struct {
	path string
	file os.FileInfo
}

var (
	singleThread  bool = false
	delete        bool = false
	linkFiles     bool = false
	visitCount    int64
	fileCount     int64
	dupCount      int64
	dupSize       int64
	minSize       int64
	hashNumBytes  int64 = 4096
	filenameMatch       = "*"
	filenameRegex *regexp.Regexp
	duplicates    = struct {
		sync.RWMutex
		m map[string][]string
	}{m: make(map[string][]string)}
	printStats   bool
	walkProgress *Progress
	walkFiles    map[int64][]*WalkedFile
)

func scanAndHashFile(path string, f os.FileInfo, progress *Progress) {
	if !f.IsDir() && f.Size() > minSize && (filenameMatch == "*" || filenameRegex.MatchString(f.Name())) {
		atomic.AddInt64(&fileCount, 1)
		file, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		} else {
			md5 := md5.New()
			_, err := io.CopyN(md5, file, hashNumBytes)
			if err != nil {
				log.Errorln(err)
			}
			var hash = fmt.Sprintf("%x", md5.Sum(nil))
			err = file.Close()
			if err != nil {
				log.Errorln(err)
			}
			duplicates.Lock()
			duplicates.m[hash] = append(duplicates.m[hash], path)
			duplicates.Unlock()
			progress.increment()
		}
	}
}

func worker(workerID int, jobs <-chan *WalkedFile, results chan<- int, progress *Progress) {
	for file := range jobs {
		fmt.Println("hashing ", file.path, " on worker ", workerID)
		scanAndHashFile(file.path, file.file, progress)
		results <- 0
	}
}

func computeHashes() {
	walkProgress := creatProgress("Scanning %d files ...", &printStats)
	jobs := make(chan *WalkedFile, visitCount)
	results := make(chan int, visitCount)

	if singleThread {
		fmt.Println("Single Thread Mode")
		go worker(1, jobs, results, walkProgress)
	} else {
		for w := 1; w <= runtime.NumCPU(); w++ {
			go worker(w, jobs, results, walkProgress)
		}
	}

	for size, files := range walkFiles {
		if len(files) > 1 {
			dupSize += size * int64(len(files)-1)
			for _, f := range files {
				jobs <- f
			}
		}
	}

	close(jobs)

	for range walkFiles {
		<-results
	}
	walkProgress.delete()
}

func visitFile(path string, f os.FileInfo, err error) error {
	visitCount++
	if !f.IsDir() && f.Size() > minSize && (filenameMatch == "*" || filenameRegex.MatchString(f.Name())) {
		walkFiles[f.Size()] = append(walkFiles[f.Size()], &WalkedFile{path, f})
		walkProgress.increment()
	}
	return nil
}

func deleteFile(path string) {
	fmt.Println("Deleting " + path)
	err := os.Remove(path)
	if err != nil {
		fmt.Printf("Error deleting file: %s \n", path)
	}
}

func linkFile(source, target string) {
	fmt.Println("Linking " + target + " to " + source)
	targetTemp := target + "-linkTemp"

	err := os.Rename(target, targetTemp)
	if err != nil {
		fmt.Printf("Error renaming file before linking: %s \n", target)
		return
	}

	err = os.Link(source, target)
	if err != nil {
		fmt.Printf("Error creating link: %s ... reverting rename\n", target)
		err2 := os.Rename(target, targetTemp)
		if err2 != nil {
			fmt.Printf("Error rverting file renaming before linking: %s \n", target)
		}
		return
	}

	err = os.Remove(targetTemp)
	if err != nil {
		fmt.Printf("Error link temp file: %s \n", targetTemp)
	}

}

func parseFlags() string {
	flag.Int64Var(&minSize, "size", 65556, "Minimum size in bytes for a file")
	flag.StringVar(&filenameMatch, "name", "*", "Filename pattern")
	flag.BoolVar(&printStats, "nostats", false, "Do no output stats")
	flag.BoolVar(&singleThread, "singleThread", false, "Work on only one thread")
	flag.BoolVar(&delete, "delete", false, "Delete duplicate files")
	flag.BoolVar(&linkFiles, "link", false, "Create hard links to duplicate files")
	var help = flag.Bool("h", false, "Display this message")
	flag.Parse()
	if *help {
		fmt.Println("duplicates is a command line tool to find duplicate files in a folder")
		fmt.Println("usage: duplicates [options...] path")
		flag.PrintDefaults()
		os.Exit(0)
	}
	printStats = !printStats // flip to positive meaning to clearify code further down
	if len(flag.Args()) < 1 {
		fmt.Fprintf(os.Stderr, "You have to specify at least a directory to explore ...\n")
		fmt.Fprintf(os.Stdout, "Run 'duplicates -h' for help\n")
		os.Exit(-1)
	}
	return flag.Arg(0)
}

func generateFileList(root string) {
	walkProgress = creatProgress("Walking through %d files ...", &printStats)
	if printStats {
		fmt.Printf("\nSearching duplicates in '%s' with name that match '%s' and minimum size '%d' bytes\n\n", root, filenameMatch, minSize)
	}
	r, _ := regexp.Compile(filenameMatch)
	filenameRegex = r
	err := filepath.Walk(root, visitFile)
	if err != nil {
		log.Errorln(err)
	}
	walkProgress.delete()
}

func ByteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}

func main() {
	walkFiles = make(map[int64][]*WalkedFile)
	//TODO: add time run time measurement

	location := parseFlags()

	generateFileList(location)
	computeHashes()
	for _, v := range duplicates.m {
		if len(v) > 1 {
			dupCount++
		}
	}

	if printStats {
		fmt.Printf("\nFound %d duplicates from %d files in %s with options { size: '%d', name: '%s' }\n", dupCount, fileCount, location, minSize, filenameMatch)
	}

	fmt.Printf("/n /n /n")
	for _, v := range duplicates.m {
		if len(v) > 1 {
			for i, file := range v {
				if i > 0 && delete {
					deleteFile(file)
					continue
				}

				if i > 0 && linkFiles {
					linkFile(v[0], file)
					continue
				}

				fmt.Printf("%s\n", file)
			}
			fmt.Println("---------")
		}
	}

	if printStats {
		fmt.Printf("\nFound %d duplicates with a total size of %s from %d files in %s with options { size: '%d', name: '%s' }\n", dupCount, ByteCountSI(dupSize), fileCount, location, minSize, filenameMatch)
	}
	os.Exit(0)
}
