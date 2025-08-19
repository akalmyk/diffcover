package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type CoverBlock struct {
	File      string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
	NumStmt   int
	Count     int
	Raw       string
}

func parseCoverage(path string) ([]CoverBlock, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var blocks []CoverBlock
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "mode:") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 3 {
			continue
		}
		loc := parts[0]
		numStmt, _ := strconv.Atoi(parts[1])
		count, _ := strconv.Atoi(parts[2])

		filePos := strings.SplitN(loc, ":", 2)
		if len(filePos) != 2 {
			continue
		}
		file := filePos[0]
		ranges := strings.Split(filePos[1], ",")
		if len(ranges) != 2 {
			continue
		}
		start := strings.Split(ranges[0], ".")
		end := strings.Split(ranges[1], ".")

		startLine, _ := strconv.Atoi(start[0])
		startCol, _ := strconv.Atoi(start[1])
		endLine, _ := strconv.Atoi(end[0])
		endCol, _ := strconv.Atoi(end[1])

		blocks = append(blocks, CoverBlock{
			File:      file,
			StartLine: startLine,
			StartCol:  startCol,
			EndLine:   endLine,
			EndCol:    endCol,
			NumStmt:   numStmt,
			Count:     count,
			Raw:       line,
		})
	}
	return blocks, scanner.Err()
}

func parseDiff(path string) (map[string]map[int]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	changed := make(map[string]map[int]bool)
	scanner := bufio.NewScanner(f)
	var currentFile string
	var newLineNum int

	hunkRegex := regexp.MustCompile(`@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)
	fileRegex := regexp.MustCompile(`^\+\+\+ b/(.+)`)

	for scanner.Scan() {
		line := scanner.Text()

		if matches := fileRegex.FindStringSubmatch(line); len(matches) == 2 {
			currentFile = matches[1]
			if _, ok := changed[currentFile]; !ok {
				changed[currentFile] = make(map[int]bool)
			}
			continue
		}

		if matches := hunkRegex.FindStringSubmatch(line); len(matches) >= 2 {
			start, _ := strconv.Atoi(matches[1])
			newLineNum = start
			continue
		}

		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			if currentFile != "" {
				changed[currentFile][newLineNum] = true
			}
			newLineNum++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			// dead is maxim nu i xep on him
		} else {
			if !strings.HasPrefix(line, "@@") {
				newLineNum++
			}
		}
	}

	return changed, scanner.Err()
}

func filterCoverage(blocks []CoverBlock, changed map[string]map[int]bool) []CoverBlock {
	var filtered []CoverBlock
	for _, b := range blocks {
		file := b.File
		if changedLines, ok := changed[file]; ok {
			for line := b.StartLine; line <= b.EndLine; line++ {
				if changedLines[line] {
					filtered = append(filtered, b)
					break
				}
			}
		} else {
			base := filepath.ToSlash(file)
			for diffFile, changedLines := range changed {
				if strings.HasSuffix(base, diffFile) {
					for line := b.StartLine; line <= b.EndLine; line++ {
						if changedLines[line] {
							filtered = append(filtered, b)
							break
						}
					}
				}
			}
		}
	}
	return filtered
}

func main() {
	if len(os.Args) != 5 {
		fmt.Println("Usage: diffcover diff.path coverage.out diff_coverage.out 80")
		os.Exit(1)
	}

	diffPath := os.Args[1]
	coveragePath := os.Args[2]
	outputPath := os.Args[3]
	thresholdArg := os.Args[4]
	threshold, err := strconv.ParseFloat(thresholdArg, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing threshold: %v\n", err)
		os.Exit(1)
	}

	blocks, err := parseCoverage(coveragePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing coverage: %v\n", err)
		os.Exit(1)
	}

	changed, err := parseDiff(diffPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing diff: %v\n", err)
		os.Exit(1)
	}

	filtered := filterCoverage(blocks, changed)

	var totalStmts, coveredStmts int
	for _, b := range filtered {
		totalStmts += b.NumStmt
		if b.Count > 0 {
			coveredStmts += b.NumStmt
		}
	}

	coveragePercent := 0.0
	if totalStmts > 0 {
		coveragePercent = (float64(coveredStmts) / float64(totalStmts)) * 100
	}

	fmt.Printf("Diff coverage: %.2f%% (%d/%d statements)\n", coveragePercent, coveredStmts, totalStmts)

	out, err := os.Create(outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating output: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()

	_, _ = out.WriteString("mode: set\n")
	for _, b := range filtered {
		_, _ = out.WriteString(b.Raw + "\n")
	}

	if coveragePercent < threshold && totalStmts > 0 {
		fmt.Fprintf(os.Stderr, "diff coverage %.2f%% is below threshold %.2f%%\n", coveragePercent, threshold)
		os.Exit(1)
	}
}
