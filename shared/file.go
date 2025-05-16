package shared

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const ChunkSize = 8192 // 8KB chunks

func EncodeFileToChunks(filePath string) ([]FileMessage, error) {
	// Check if file exists
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}

	// Check if file is empty
	totalSize := fileInfo.Size()
	if totalSize == 0 {
		return nil, fmt.Errorf("file is empty: %s", filePath)
	}

	fileName := filepath.Base(filePath)
	totalChunks := int((totalSize + ChunkSize - 1) / ChunkSize)

	chunks := make([]FileMessage, 0, totalChunks)

	buffer := make([]byte, ChunkSize)
	chunkID := 0

	for {
		bytesRead, err := file.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if bytesRead == 0 {
			break
		}

		encodedData := base64.StdEncoding.EncodeToString(buffer[:bytesRead])

		chunk := FileMessage{
			Message: Message{
				Type:      MessageTypeFile,
				Timestamp: time.Now(),
			},
			Filename:    fileName,
			Size:        totalSize,
			ChunkID:     chunkID,
			TotalChunks: totalChunks,
			Data:        []byte(encodedData),
		}

		chunks = append(chunks, chunk)
		chunkID++
	}

	return chunks, nil
}

func SaveFileFromChunks(chunks []FileMessage, outputDir string) error {
	if len(chunks) == 0 {
		return nil
	}

	fileName := chunks[0].Filename
	outputPath := filepath.Join(outputDir, fileName)

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Sort chunks by ChunkID
	sortedChunks := make([]FileMessage, len(chunks))
	copy(sortedChunks, chunks)
	sort.Slice(sortedChunks, func(i, j int) bool {
		return sortedChunks[i].ChunkID < sortedChunks[j].ChunkID
	})

	for _, chunk := range sortedChunks {
		data, err := base64.StdEncoding.DecodeString(string(chunk.Data))
		if err != nil {
			return err
		}

		_, err = file.Write(data)
		if err != nil {
			return err
		}
	}

	return nil
}
