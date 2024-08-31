package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"io"
)

type DownloadChunk struct {
    Start uint64
    End uint64
};

/*
Attempts to get the file size of the download.

Arguments:
    - url (string): The url of the resource to download.

Returns:
    - int: The size of the file to download.
    - error: The error if any occured.

Example:
    size, err := GetFileSize("https://google.com");
    if err != nil {
        return err;
    }
*/
func GetFileSize(url string) (uint64, error) {
    resp, err := http.Head(url);
    if err != nil {
        return 0, fmt.Errorf("Error: Failed to make request to %s. %w", url, err);
    }
    defer resp.Body.Close();

    contentSize := resp.Header["Content-Length"];
    if contentSize == nil {
        return 0, fmt.Errorf("Error: Failed to determine download size.");
    }

    size, err := strconv.ParseInt(contentSize[0], 10, 64);
    if err != nil {
        return 0, fmt.Errorf("Error: Failed to determine download size. %w", err);
    }

    return uint64(size), nil;
}

/*
Returns a list of chunks that need to be downloaded.

Arguments:
    - size (uint64): The total size of the download.
    - threads (uint64): The number of threads to run.

Returns:
    []DownloadChunk: The list of chunks that need to be downloaded.

Example:
    chunks := GetDownloadChunks(1024, 4);
*/
func GetDownloadChunks(size uint64, threads uint64) []DownloadChunk {
    chunkSize := size / threads;
    results := make([]DownloadChunk, threads);

    var i uint64;
    for i = 0; i < threads; i++ {
        if i == threads - 1 {
            results[i] = DownloadChunk{
                Start: i * chunkSize,
                End: (i * chunkSize) + (size - (i * chunkSize)),
            };
        } else {
            results[i] = DownloadChunk{
                Start: i * chunkSize,
                End: (i * chunkSize) + chunkSize,
            };
        }
    }

    return results;
}

/*
Downloads the requested chunks from the url.

Arguments:
    - url (string): The url of what to download.
    - chunks ([]DownloadChunks): The chunks to download.
    - output (string): The path to where to save the downloaded file.

Returns:
    - error: The error if any occured.

Example:
    err := DownloadChunks("https://google.com", chunks, "./google.html")
    if err != nil {
        return err;
    }
*/
func DownloadChunks(url string, chunks []DownloadChunk, output string) error {
    f, err := os.Create(output);
    if err != nil {
        return fmt.Errorf("Error: Failed to create file %s. %v", output, err);
    }
    defer f.Close();

    var mutex sync.Mutex;
    errChan := make(chan error);
        
    for i := 0; i < len(chunks); i++ {
        go func(start uint64, end uint64) {
            fmt.Printf("Log: Downloading chuck %d-%d.\n", start, end);
            req, err := http.NewRequest("GET", url, nil);
            if err != nil {
                errChan<-fmt.Errorf("Error: Failed to create request for %s. %v", url, err);
                return;
            }
            chunkHeader := fmt.Sprintf("bytes=%d-%d", start, end);
            req.Header.Set("Range", chunkHeader);

            resp, err := http.DefaultClient.Do(req);
            if err != nil {
                errChan<-fmt.Errorf("Error: Failed to make request to %s. %v", url, err);
                return;
            }
            defer resp.Body.Close();
            if resp.StatusCode != http.StatusPartialContent {
                errChan<-fmt.Errorf("Error: Unsuccessful status code form %s (%d).", url, resp.StatusCode);
                return;
            }

            mutex.Lock();
            fmt.Printf("Log: Writing chuck %d-%d.\n", start, end);
            _, err = f.Seek(int64(start), 0);
            if err != nil {
                errChan<-fmt.Errorf("Error: Unsuccessful writing to file %s. %v", output, err);
                mutex.Unlock();
                return;
            }

            _, err = io.Copy(f, resp.Body);
            if err != nil {
                errChan<-fmt.Errorf("Error: Unsuccessful writing to file %s. %v", output, err);
                mutex.Unlock();
                return;
            }
            mutex.Unlock();
            errChan<-nil;
        }(chunks[i].Start, chunks[i].End)
    }

    for i := 0; i < len(chunks); i++ {
        err := <-errChan;
        if err != nil {
            return err;
        }
    }

    return nil;
}

func main() {
    if len(os.Args) < 3 {
        fmt.Printf("Usage: gofast <url> <threads> <output file path>\n");
        return;
    } 

    downloadSize, err := GetFileSize(os.Args[1]);
    if err != nil {
        fmt.Fprintf(os.Stderr, "%v\n", err);
        return;
    }
    fmt.Printf("Log: Download size: %d bytes.\n", downloadSize);

    t, err := strconv.ParseInt(os.Args[2], 10, 64);
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: Invalid thread number. %v\n", err);
        return;
    }
    if t <= 0 {
        fmt.Fprintf(os.Stderr, "Error: Invalid thread number.\n");
        return;
    }
    threads := uint64(t);

    chunks := GetDownloadChunks(downloadSize, threads);
    err = DownloadChunks(os.Args[1], chunks, os.Args[3]);
    if err != nil {
        fmt.Fprintf(os.Stderr, "%v\n", err);
    }
}
