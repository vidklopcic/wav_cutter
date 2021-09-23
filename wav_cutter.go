package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

var wg sync.WaitGroup

var fromDir *string
var toDir *string
var maxRatioError *float64
var maxError *float64
var offsetEnd *float64
var offsetStart *float64
var verbose *bool
var splitInHalf *bool
var deleteSourceError *float64
var deleteSourceRatioError *float64
var srcFileIndex *int
var startSIndex *int
var endSIndex *int
var outFileIndex *int
var diffErrorIndex *int
var ratioErrorIndex *int

func main() {
	var trimArgsCsv = flag.String("csv", "", "CSV file containing arguments for cutting [src file, start [s], end [s], out file]")
	fromDir = flag.String("from", "", "Dir of source files")
	toDir = flag.String("to", "", "Destination dir")
	maxRatioError = flag.Float64("max-ratio-error", -1, "Maximum ratio error to cut (skips if abs(1-ratio) > max-ratio-error)")
	maxError = flag.Float64("max-error", -1, "Max diff error to cut")
	offsetEnd = flag.Float64("offset-end", 0, "Offset in seconds to add at the end of cut")
	offsetStart = flag.Float64("offset-start", 0, "Offset in seconds to add at the start of cut")
	deleteSourceError = flag.Float64("delete-source-error", -1, "Delete files that were successfully cut and have error lte than specified.")
	deleteSourceRatioError = flag.Float64("delete-source-ratio-error", -1, "Delete files that were successfully cut and have ratio error lte than specified")
	verbose = flag.Bool("verbose", false, "Print all errors")
	splitInHalf = flag.Bool("split-in-half", false, "Split each file in half and outpu a_<fn>.wav and b_<fn>.wav")
	srcFileIndex = flag.Int("src-file-index", 0, "cut-args csv index for srcFile")
	startSIndex = flag.Int("start-s-index", 1, "cut-args csv index for startS")
	endSIndex = flag.Int("end-s-index", 2, "cut-args csv index for endS")
	outFileIndex = flag.Int("out-file-index", 3, "cut-args csv index for outFile")
	diffErrorIndex = flag.Int("error-index", 4, "cut-args csv index for diffError")
	ratioErrorIndex = flag.Int("ratio-error-index", 5, "cut-args csv index for ratioError")
	flag.Parse()
	fmt.Println("started using:")
	fmt.Println("max-error", *maxError)
	fmt.Println("max-ratio-error", *maxRatioError)
	fmt.Println("delete-source-error", *deleteSourceError)
	fmt.Println("delete-source-ratio-error", *deleteSourceRatioError)
	fmt.Println("-----------------------")

	file, err := os.Open(*trimArgsCsv)
	defer file.Close()
	if err != nil {
		fmt.Println("file", *trimArgsCsv, "doesn't exist")
		return
	}

	reader := csv.NewReader(file)
	readerChannel := make(chan []string)
	for i := 0; i < 10; i++ {
		go cutRoutine(readerChannel)
		wg.Add(1)
	}

	n := 0
	for {
		records, err := reader.Read()
		if err != nil {
			fmt.Println(err)
			break
		}
		readerChannel <- records
		n++
		if n%1000 == 0 {
			fmt.Print("processing: ", n, "\r")
		}
	}
	close(readerChannel)
	wg.Wait()
	fmt.Println("done")
}

func cutRoutine(channel chan []string) {
	cutWriter := WavCopyWriter{}
	defer wg.Done()
	for {
		select {
		case entry, ok := <-channel:
			if !ok {
				fmt.Println("Exiting goroutine")
				return
			}

			start_s, err := strconv.ParseFloat(entry[*startSIndex], 32)
			if err != nil {
				fmt.Println("error parsing start_s for", entry[*srcFileIndex])
				continue
			}
			end_s, err := strconv.ParseFloat(entry[*endSIndex], 32)
			if err != nil {
				fmt.Println("error parsing end_s for", entry[*srcFileIndex])
				continue
			}

			hasDiffError := len(entry) > *diffErrorIndex
			hasRatioError := len(entry) > *ratioErrorIndex
			var diffError float64
			var ratioError float64
			if hasDiffError {
				// check diffError
				diffError, err = strconv.ParseFloat(entry[*diffErrorIndex], 32)
				if err != nil {
					fmt.Println("error parsing diffError for", entry[*srcFileIndex])
					continue
				}
				if *maxError > 0 && diffError > *maxError {
					continue
				}
			}

			if hasRatioError {
				// check ratio
				ratio, err := strconv.ParseFloat(entry[*ratioErrorIndex], 32)
				ratioError = math.Abs(1 - ratio)
				if err != nil {
					fmt.Println("diffError parsing ratio for", entry[*srcFileIndex])
					continue
				}
				if *maxRatioError > 0 && ratioError > *maxRatioError {
					continue
				}
			}

			start_s += *offsetStart
			end_s += *offsetEnd
			if *splitInHalf {
				dur_s := end_s - start_s
				startA, endA := start_s, start_s+dur_s/2
				startB, endB := endA, end_s

				cutWriter.source = filepath.Join(*fromDir, entry[*srcFileIndex])
				cutWriter.start = float32(startA)
				cutWriter.end = float32(endA)
				cutWriter.dest = filepath.Join(*toDir, "a_"+entry[*outFileIndex])
				err = cutWriter.write()
				if err != nil && *verbose {
					fmt.Println(entry, err)
				}

				cutWriter.source = filepath.Join(*fromDir, entry[*srcFileIndex])
				cutWriter.start = float32(startB)
				cutWriter.end = float32(endB)
				cutWriter.dest = filepath.Join(*toDir, "b_"+entry[*outFileIndex])
				err = cutWriter.write()
				if err != nil && *verbose {
					fmt.Println(entry, err)
				}
			} else {
				cutWriter.source = filepath.Join(*fromDir, entry[*srcFileIndex])
				cutWriter.start = float32(start_s + *offsetStart)
				cutWriter.end = float32(end_s + *offsetEnd)
				cutWriter.dest = filepath.Join(*toDir, entry[*outFileIndex])
				err = cutWriter.write()
				if err != nil && *verbose {
					fmt.Println(entry, err)
				}
			}

			if err == nil && (hasDiffError && hasRatioError && diffError <= *deleteSourceError && ratioError <= *deleteSourceRatioError) {
				err = os.Remove(cutWriter.source)
				if err != nil && *verbose {
					fmt.Println(entry, err)
				}
			}
		}
	}
}
