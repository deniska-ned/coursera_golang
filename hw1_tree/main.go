package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

func dirTreeRec(out io.Writer, path string, printFiles bool, dirPrefix string) (ferr error) {
	files, err := ioutil.ReadDir(path)

	if err != nil {
		ferr = err
		return
	} else {
		i_lastprinted := -1
		for i, file := range files {
			if file.IsDir() || printFiles {
				i_lastprinted = i
			}
		}

		dirChildPrefix := dirPrefix + "├───"
		childDirPrefix := dirPrefix + "│\t"

		for i, file := range files {
			if i == i_lastprinted {
				dirChildPrefix = dirPrefix + "└───"
				childDirPrefix = dirPrefix + "\t"
			}

			if file.IsDir() {
				fmt.Fprintf(out, "%s%s\n", dirChildPrefix, file.Name())

				dirTreeRec(out, filepath.Join(path, file.Name()), printFiles, childDirPrefix)
			} else {
				if printFiles {
					info, _ := os.Stat(filepath.Join(path, file.Name()))
					filesize := strconv.FormatInt(info.Size(), 10)
					if "0" == filesize {
						filesize = "empty"
					} else {
						filesize += "b"
					}
					fmt.Fprintf(out, "%s%s (%s)\n", dirChildPrefix, file.Name(), filesize)
				}
			}
		}
	}

	return
}

func dirTree(out io.Writer, path string, printFiles bool) (ferr error) {
	ferr = dirTreeRec(out, path, printFiles, "")
	return
}

func main() {
	out := os.Stdout
	if !(len(os.Args) == 2 || len(os.Args) == 3) {
		panic("usage go run main.go . [-f]")
	}
	path := os.Args[1]
	printFiles := len(os.Args) == 3 && os.Args[2] == "-f"
	err := dirTree(out, path, printFiles)
	if err != nil {
		panic(err.Error())
	}
}
