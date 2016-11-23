package utils

import (
	"bufio"
	"io"
	"os"
)

func ReadFlistFile(path string) ([]string, error) {
	flistFile, err := os.Open(path)
	defer flistFile.Close()
	if err != nil {
		log.Errorf("Error opening flist %s :%v", path, err)
		return nil, err
	}
	metadata := []string{}
	scanner := bufio.NewScanner(flistFile)
	var line string
	for scanner.Scan() {
		line = scanner.Text()
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		metadata = append(metadata, line)
	}
	return metadata, nil
}

func IterFlistFile(path string) (<-chan string, error) {
	file, err := os.Open(path)
	if err != nil {
		log.Errorf("Error opening flist %s :%v", path, err)
		return nil, err
	}

	c := make(chan string)
	go func(c chan string, file io.ReadCloser) {
		defer close(c)
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			c <- line
		}

		if err := scanner.Err(); err != nil {
			log.Errorf("Error while scanning file '%s': %s", path, err)
			return
		}
	}(c, file)

	return c, nil
}
