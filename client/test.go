package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	_ "encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	_ "strings"
	"time"
)

type ReturnFiles struct {
	Status          int
	Msg             string
	NumDeletedFiles int
	DeletedFiles    map[string]*FileInfo
}
type FileServerInfo struct {
	Status     int
	Msg        string
	ID         string
	FileNumber int
}
type UploadResponseInfo struct {
	Status       int
	Msg          string
	DownloadPath string
}
type FileInfo struct {
	CreateTime  time.Time
	Md5         string
	ExpiredTime time.Time
}
type FileInfoResponse struct {
	Status  int
	Msg     string
	File    FileInfo
	AllFile []string
}

// Creates a new file upload http request with optional extra params
func newfileUploadRequestWithGzip(uri string, params map[string]string, paramName, path string) (*http.Request, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	//gzip compress file
	// gzip writes to pipe, http reads from pipe
	pr, pw := io.Pipe()

	// buffer readers from file, writes to pipe
	bufin := bufio.NewReader(file)

	// gzip wraps buffer writer and wr
	gw := gzip.NewWriter(pw)

	// Actually start reading from the file and writing to gzip
	go func() {
		fmt.Println("Start writing")
		n, err := bufin.WriteTo(gw)
		if err != nil {
			fmt.Println(err)
		}
		gw.Close()
		pw.Close()
		fmt.Printf("Done writing: %d", n)
	}()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(paramName, filepath.Base(path))
	if err != nil {
		return nil, err
	}
	//not compress file
	//_, err = io.Copy(part, file)

	//compress file
	_, err = io.Copy(part, pr)

	for key, val := range params {
		_ = writer.WriteField(key, val)
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", uri, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Content-Encoding", "gzip")
	return req, err
}

func newfileUploadRequestWithoutGzip(uri string, params map[string]string, paramName, path string) (*http.Request, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(paramName, filepath.Base(path))
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)
	for key, val := range params {
		_ = writer.WriteField(key, val)
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", uri, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, err
}

func main() {
	/*//get status
	resp, err4 := http.Get("http://localhost:50010/r/status")
	if err4 != nil {
		fmt.Println(err4)
		return
	}
	defer resp.Body.Close()
	var fileInfo FileServerInfo
	err2 := json.NewDecoder(resp.Body).Decode(&fileInfo)
	if err2 != nil {
		fmt.Println("error")
		return
	}
	fmt.Println(fileInfo)*/

	/*//clean
	resp, err4 := http.Get("http://localhost:50010/r/clean")
	if err4 != nil {
		fmt.Println(err4)
		return
	}
	defer resp.Body.Close()
	var temp ReturnFiles
	err2 := json.NewDecoder(resp.Body).Decode(&temp)
	if err2 != nil {
		fmt.Println("error")
		return
	}
	fmt.Println(temp)*/

	/*//get all files of a folder
	req, err := http.NewRequest("GET", "http://localhost:50010/r/info/", nil)
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}

	q := req.URL.Query()
	q.Add("isDir", "true")
	q.Add("recursion", "true")
	q.Add("suffix", ".log")
	req.URL.RawQuery = q.Encode()

	fmt.Println(req.URL.String())

	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if err != nil {

	}
	defer resp.Body.Close()
	var fileInfo FileInfoResponse
	err2 := json.NewDecoder(resp.Body).Decode(&fileInfo)
	if err2 != nil {
		fmt.Println("error")
		return
	}
	fmt.Println(fileInfo)
	fmt.Printf("123.log md5 checksum is: %x", fileInfo.File.Md5)*/

	/*
		//get info of a file
		resp, err4 := http.Get("http://localhost:50010/r/info/dany/123.log")
		if err4 != nil {
			fmt.Println(err4)
			return
		}
		defer resp.Body.Close()
		var fileInfo FileInfoResponse
		err2 := json.NewDecoder(resp.Body).Decode(&fileInfo)
		if err2 != nil {
			fmt.Println("error")
			return
		}
		fmt.Println(fileInfo)
		fmt.Printf("123.log md5 checksum is: %x", fileInfo.File.Md5)
	*/
	/*
		//upload file
		extraParams := map[string]string{
			"expiredTime": "300ms",
			"dest":   "/456.log/",
			"replaceIfExist": "1",
		}
		request, err := newfileUploadRequestWithGzip("http://localhost:50010/r/upload/", extraParams, "file", "./456.log")
		if err != nil {
			fmt.Println(err)
		}
		client := &http.Client{}
		resp, err := client.Do(request)
		if err != nil {
			fmt.Println(err)
		} else {
			defer resp.Body.Close()
			fmt.Println(resp.StatusCode)
			fmt.Println(resp.Header)
			var info UploadResponseInfo
			err := json.NewDecoder(resp.Body).Decode(&info)
			if err != nil {
				fmt.Println("error")
				return
			}
			fmt.Println(info)
		}
	*/
	//download file

	url := "http://localhost:50010/r/download/dany/123.log"
	client2 := new(http.Client)
	request2, err := http.NewRequest("GET", url, nil)
	request2.Header.Add("Accept-Encoding", "gzip")

	resp, err4 := client2.Do(request2)
	if err4 != nil {
		fmt.Println("Error while send download request", url, "-", err)
		return
	}
	defer resp.Body.Close()

	body := &bytes.Buffer{}
	_, err2 := body.ReadFrom(resp.Body)
	if err2 != nil {
		fmt.Println(err)
	}
	tokens := strings.Split(url, "/")
	fileName := tokens[len(tokens)-1]
	// TODO: check file existence first with io.IsExist
	output, err := os.Create(fileName)
	if err != nil {
		fmt.Println("Error while creating", fileName, "-", err)
		return
	}
	defer output.Close()

	fmt.Println("+++++++", resp.Header.Get("Content-Encoding"))
	fmt.Println(resp.StatusCode)
	fmt.Println(resp.Uncompressed)
	fmt.Println(resp.Header)
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		fmt.Println("enter gzip +++++++++++++")
		rr, err := gzip.NewReader(body)
		if err != nil {
			fmt.Println(err)
		}
		defer rr.Close()
		io.Copy(output, rr)
	default:
		fmt.Println("------------")
		rr := body
		io.Copy(output, rr)
	}
}
