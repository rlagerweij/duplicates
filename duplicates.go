package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
)

var ( // these variable as set at build time
	sha1ver   string = "testing" // sha1 revision used to build the program
	buildTime string = "testing" // when the executable was built
)

// WalkedFile a type of struct
type WalkedFile struct {
	path string
	file os.FileInfo
}

type PotentialDupe struct {
	WalkedFile
	quickHash string
	fullHash  string
}

var (
	singleThread      bool = false
	delete            bool = false
	linkFiles         bool = false
	hashEntireFile    bool = false
	visitCount        int64
	fileCount         int64
	dupCount          int64
	dupSize           int64
	potentialDupCount int64
	minSize           int64
	maxSize           int64
	hashNumBytes      int64 = 4096
	filenameMatch           = "*"
	filenameRegex     *regexp.Regexp
	duplicates        = struct {
		sync.RWMutex
		m map[int64][]*PotentialDupe
	}{m: make(map[int64][]*PotentialDupe)}
	printStats   bool
	walkProgress *Progress
	walkFiles    = make(map[int64][]*WalkedFile)
)

var wg sync.WaitGroup

func main() {

	fmt.Printf("Duplicates (version: %s / %s)\n", sha1ver, buildTime)

	startTime := time.Now()
	location := parseFlags()
	generateFileList(location)

	countFilesWithSameSize()

	fmt.Printf("\nFound %d files with %d potential duplicates in: %s\n", visitCount, potentialDupCount, location)

	if potentialDupCount == 0 {
		fmt.Println("No duplicates found")
		os.Exit(0)
	}

	computeHashes()

	processResults()

	if dupCount == 0 {
		fmt.Println("No duplicates found")
		os.Exit(0)
	}

	fmt.Println("---------")

	duration := time.Since(startTime)

	fmt.Printf("\n%d duplicates found with a total size of %s from %d files investigated in %s\n", dupCount, ByteCountSI(dupSize), fileCount, duration.String())

	os.Exit(0)

}

func scanAndHashFile(path string, f os.FileInfo, progress *Progress) {
	defer wg.Done()
	if !f.IsDir() && f.Size() > minSize && (filenameMatch == "*" || filenameRegex.MatchString(f.Name())) {
		atomic.AddInt64(&fileCount, 1)
		file, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		} else {
			md5 := md5.New()

			if hashEntireFile {
				_, err := io.Copy(md5, file)
				if err != nil {
					log.Errorln(err)
				}

			} else {
				_, err := io.CopyN(md5, file, hashNumBytes)
				if err != nil {
					log.Errorln(err)
				}

				file.Seek(hashNumBytes*-1, io.SeekEnd)
				_, err = io.CopyN(md5, file, hashNumBytes)

				if err != nil {
					log.Errorln(err)
				}
			}
			var quickHash = fmt.Sprintf("%x", md5.Sum(nil))
			err = file.Close()
			if err != nil {
				log.Errorln(err)
			}
			duplicates.Lock()
			duplicates.m[f.Size()] = append(duplicates.m[f.Size()], &PotentialDupe{WalkedFile{path, f}, quickHash, string("")})
			duplicates.Unlock()
			progress.increment()
		}
	}
}

func hash_worker(workerID int, jobs <-chan *WalkedFile, progress *Progress) {
	for file := range jobs {
		if printStats {
			fmt.Println("Hashing ", file.path, " on worker ", workerID)
		}
		scanAndHashFile(file.path, file.file, progress)
	}
}

func computeHashes() {
	walkProgress := creatProgress("Scanning %d files ...", &printStats)
	jobs := make(chan *WalkedFile, visitCount)

	if singleThread {
		fmt.Println("Single Thread Mode")
		go hash_worker(1, jobs, walkProgress)
	} else {
		for w := 1; w <= runtime.NumCPU(); w++ {
			go hash_worker(w, jobs, walkProgress)
		}
	}

	for _, files := range walkFiles {
		if len(files) > 1 {
			for _, f := range files {
				wg.Add(1)
				jobs <- f
			}
		}
	}

	close(jobs)

	wg.Wait()

	walkProgress.delete()
}

func visitFile(path string, f os.FileInfo, err error) error {
	visitCount++
	if !f.IsDir() && f.Size() > minSize && f.Size() <= maxSize && (filenameMatch == "*" || filenameRegex.MatchString(f.Name())) {
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
	flag.Int64Var(&minSize, "min-size", 65556, "Minimum size in bytes for a file")
	flag.Int64Var(&maxSize, "max-size", math.MaxInt, "Maximum size in bytes for a file")
	flag.StringVar(&filenameMatch, "name", "*", "Filename pattern")
	flag.BoolVar(&printStats, "nostats", false, "Do no output stats")
	flag.BoolVar(&singleThread, "singleThread", false, "Work on only one thread")
	flag.BoolVar(&delete, "delete", false, "Delete duplicate files")
	flag.BoolVar(&linkFiles, "link", false, "Create hard links to duplicate files")
	flag.BoolVar(&hashEntireFile, "full", false, "Hash the entrire contents of suspected duplicate files (slower)")
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
		os.Exit(0)
	}

	return flag.Arg(0)
}

func generateFileList(root string) {
	walkProgress = creatProgress("Walking through %d files ...", &printStats)

	fmt.Printf("\nSearching duplicates in '%s' with name that matches '%s' and minimum size of '%d' bytes\n", root, filenameMatch, minSize)

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

func processResults() {
	fmt.Print("\nProcessing results:\n")

	for s, v := range duplicates.m {

		fmt.Println("---------")

		if len(v) > 1 {
			for i, file := range v {
				fmt.Printf("[%d] [%s] %s", s, file.quickHash, file.path)

				if i == 0 {
					fmt.Println()
					continue
				}

				sameHash := (file.quickHash == v[0].quickHash) && (i > 0)

				if sameHash {
					dupCount += 1
					dupSize += file.file.Size()
					fmt.Println(" [DUP]")
				} else {
					fmt.Println(" [NO-DUP]")
				}

				if sameHash && delete {

					deleteFile(file.path)
					continue
				}

				if sameHash && linkFiles {
					linkFile(v[0].path, file.path)
					continue
				}
			}
		}
	}
}

func countFilesWithSameSize() {
	for _, v := range walkFiles {
		if len(v) > 1 {
			potentialDupCount += int64(len(v) - 1)
			// potentialDupSize += k * int64(len(v)-1)
		}
	}
}
