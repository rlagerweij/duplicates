# duplicates

File duplicates finder

Sometimes you need to find duplicates files on your disk. You can use this tool to do it. It uses a MD5 hash to identify duplicate files. You can also use some options to filter files by names an minimum size (in bytes).

This program was initially designed and written by [Mathieu Ancelin](https://github.com/mathieuancelin/duplicates) and added upon by [carfloresf](https://github.com/carfloresf/duplicates). It had a great multi-threaded architecture but I wanted to adapt it to my use case, which is deduplicating a large media collection. The original program fully hashed every file to find duplicates based on hash, which is not necessary on large media files and also painfully slow. On the flip size, with partial hashes you theoretically run the risk of false duplicates, although this risk is low.

To make this program better suited to large files I made the following changes:
- Manage potential duplicates based on size (faster, reduces the number of files that need to be hashed)
- Hash only the first and last 4k block in a file (much faster for large files)
- Implement the option to hard-link duplicates, which doesn't remove them but does free up space
- Increase min file size to 64K (from 1 byte)
- Implement max file size command line option
- Add execution timing

The resulting program is very, very fast. It can scan and hash a folder with 10,200 camera images with a total size of 98 Gb in 3.2 seconds on an 2012 era quad core Xeon (folder is on an NVME ssd). A folder with multiple snapshot backups of the same photos with 223 Gb in over 50,000 files is scanned and hashed in 1m05s (>50% duplicate rate).

## usage

```
usage: duplicates [options...] path

  -h          Display the help message
  -name       Filename pattern
  -nostats    Do no output stats
  -single     Work in single threaded mode
  -min-size   Minimum size in bytes for a file (default: 64k)
  -max-size   Maximum size in bytes for a file (default: no maximum)
  -delete     Deletes duplicate files
  -link       Hard-links duplicate files
  -full       Hash the full file (safer, default: hash first and last 4k block of a file)
```

## examples

```
$ duplicates /tmp
$ duplicates -link /tmp
$ duplicates -name .mp3 /tmp
$ duplicates -min-size 1 /tmp
$ duplicates -min-size 2056 -name .mp3 /tmp
$ duplicates -nostats -size 2056 -name .mp3 /tmp > duplicates.txt
```

## install

- from source

```
go get github.com/rlagerweij/duplicates
```

- binaries

See releases on the side bar.
