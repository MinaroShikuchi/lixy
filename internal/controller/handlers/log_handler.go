// internal/controller/handlers/log_streaming.go
package handlers

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type LogHandlers struct {
	logPath string
}

func NewLogHandlers(logPath string) *LogHandlers {
	return &LogHandlers{
		logPath: logPath,
	}
}

func (lh *LogHandlers) StreamLogsHandler(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Debug message to confirm connection
	fmt.Fprintf(w, "data: Starting log streaming...\n\n")

	// Check for flusher support
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	flusher.Flush()
	fmt.Fprintf(w, "data: Flusher obtained, starting to stream logs...\n\n")
	flusher.Flush()

	// Keep track of file offset
	var offset int64 = 0

	// Create a context that will be canceled on client disconnect
	ctx := r.Context()

	// Create ticker for checking file updates
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Main streaming loop
	fmt.Fprintf(w, "data: Entering streaming loop...\n\n")
	flusher.Flush()

	for {
		select {
		case <-ticker.C:
			// Open file on every iteration to check for changes
			file, err := os.Open(lh.logPath)
			if err != nil {
				fmt.Fprintf(w, "data: Error opening log file: %s\n\n", err)
				flusher.Flush()
				continue
			}

			// Get current file size
			stat, err := file.Stat()
			if err != nil {
				file.Close()
				fmt.Fprintf(w, "data: Error getting file stats: %s\n\n", err)
				flusher.Flush()
				continue
			}

			// Check if file has been truncated (log rotation)
			if stat.Size() < offset {
				fmt.Fprintf(w, "data: Log file was truncated, resetting position\n\n")
				flusher.Flush()
				offset = 0
			}

			// Check if there's new content
			if stat.Size() > offset {
				// Seek to previous offset
				_, err = file.Seek(offset, 0)
				if err != nil {
					file.Close()
					fmt.Fprintf(w, "data: Error seeking in file: %s\n\n", err)
					flusher.Flush()
					continue
				}

				// Read new content
				reader := bufio.NewReader(file)
				newBytes := stat.Size() - offset
				fmt.Fprintf(w, "data: Found %d new bytes in log file\n\n", newBytes)
				flusher.Flush()

				// Read and send each line
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						if err == io.EOF {
							// Reached end of current content
							break
						}
						fmt.Fprintf(w, "data: Error reading file: %s\n\n", err)
						flusher.Flush()
						break
					}

					// Send the log line
					fmt.Fprintf(w, "data: %s\n\n", strings.TrimRight(line, "\n"))
					flusher.Flush()
				}

				// Update offset to current position
				offset = stat.Size()
			}

			file.Close()

		case <-ctx.Done():
			// Client disconnected
			fmt.Println("Client disconnected")
			return
		}
	}
}
