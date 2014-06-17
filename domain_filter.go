package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"regexp"
	"strings"
)

func GetFilter(file string) (*regexp.Regexp, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	bufr := bufio.NewReader(f)
	regBuf := bytes.NewBufferString("(")
	for run := true; run; {
		line, err := bufr.ReadString('\n')
		if err == io.EOF {
			run = false
		} else if err != nil {
			return nil, err
		}
		line = strings.Trim(line, " \n\r\t")
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		line = strings.Replace(line, ".", "\\.", -1)
		line = ".*\\." + line + "|" + line
		if err != io.EOF {
			line += "|"
		}
		regBuf.WriteString(line)
	}
	regBuf.WriteString(")")
	return regexp.Compile(regBuf.String())
}
