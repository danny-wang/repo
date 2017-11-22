package main

import (
	"compress/gzip"
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/julienschmidt/httprouter"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"repo/httpgzip"
	"repo/log"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	port     string
	logDir   string
	logLevel string
	dataDir  string
}
type FileInfo struct {
	CreateTime   time.Time
	Md5          string
	ExpiredTime  time.Time
	DownloadPath string `json:",omitempty"`
}
type UploadResponseInfo struct {
	ErrInfo
	File FileInfo
}
type FileServerInfo struct {
	ErrInfo
	ID         string
	FileNumber int
}
type FileInfoResponse struct {
	ErrInfo
	File    FileInfo
	AllFile []string
}

var svr = &Server{}
var db *bolt.DB
var fileNum int = 0

const DEFAULT_EXPIRED_TIME = "2400h"
/*
Upload file handler
Use blow instruction to upload file or construct post request by yourself:
curl  -F "file=@bolt" -F dest=/jianwang/bolt.txt  -F expiredTime=2h  -F replaceIfExist=false  "http://localhost:50010/r/upload/"
Return value:
{"Status":0,"Msg":"OK","File":{"CreateTime":"2017-08-15T16:03:19.257401537+08:00","Md5":"e286c3a32a578cff7b7a39dc943aa1e5",
"ExpiredTime":"2017-08-15T18:03:19.257405226+08:00","DownloadPath":"http://192.168.0.32:50011/r/download/jianwang/ads.111"}}
*/
func upload(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log.Debugf("%s: %s, From: %s, Content-Encoding: %s", r.Method, r.RequestURI, r.RemoteAddr, r.Header.Get("Content-Encoding"))
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	r.ParseMultipartForm(32 << 20)
	expiredDuration, err := time.ParseDuration(valuesGetDefault(r.Form, "expiredTime", DEFAULT_EXPIRED_TIME))
	if err != nil {
		log.Warn(err)
		json.NewEncoder(w).Encode(MakeErrInfo(ERR_REQ_PARAMETER_EXPIRE))
		return
	}
	reqPath := valuesGetDefault(r.Form, "dest", "")
	if len(reqPath) <= 1 || reqPath[0] != '/' {
		json.NewEncoder(w).Encode(MakeErrInfo(ERR_REQ_PARAMETER_PATH))
		return
	}
	downloadPath := "http://" + r.Host + "/r/download" + reqPath
	localPath := path.Join(svr.dataDir, reqPath)
	if st, err := os.Stat(localPath); err != nil {
		if !os.IsNotExist(err) {
			json.NewEncoder(w).Encode(MakeErrInfo(ERR_OPEN_FILE))
			return
		}
	} else {
		if st.IsDir() {
			json.NewEncoder(w).Encode(MakeErrInfo(ERR_FILE_EXIST_DIR))
			return
		}
		replaceIfExist := valuesGetDefault(r.Form, "replaceIfExist", "true")
		if strings.ToLower(replaceIfExist) != "true" && replaceIfExist != "1" {
			// TODO: get file info here
			json.NewEncoder(w).Encode(UploadResponseInfo{ErrInfo: ErrInfo{Status: ERR_OK,
				Msg: "file exist"}})
			return
		}
	}
	if err := os.MkdirAll(path.Dir(localPath), os.ModePerm); err != nil {
		log.Error(err)
		json.NewEncoder(w).Encode(MakeErrInfo(ERR_MKDIR))
		return
	}
	f, err := os.OpenFile(localPath, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		log.Error(err)
		json.NewEncoder(w).Encode(MakeErrInfo(ERR_OPEN_FILE))
		return
	}
	defer f.Close()

	contentIsCompressed := false
	for _, v := range r.Header["Content-Encoding"] {
		if v == "gzip" {
			contentIsCompressed = true
		}
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		json.NewEncoder(w).Encode(MakeErrInfo(ERR_HTTP_GET_CONTENT))
		return
	}
	defer file.Close()

	var size int64 = 0
	md5Writer := md5.New()
	if contentIsCompressed {
		//for gzip compressed file
		gr, err := gzip.NewReader(file)
		if err != nil {
			log.Fatal(err)
		}
		defer gr.Close()
		size, _ = io.Copy(io.MultiWriter(md5Writer, f), gr)
		log.Debugf(`Recv %v plaintext bytes size`, size)
	} else {
		//for not use gzip compressed file
		size, _ = io.Copy(io.MultiWriter(md5Writer, f), file)
		log.Debugf(`Recv %v plaintext bytes size`, size)
	}
	fileInfo := &FileInfo{
		CreateTime:  time.Now(),
		Md5:         fmt.Sprintf("%x", md5Writer.Sum(nil)),
		ExpiredTime: time.Now().Add(expiredDuration),
	}
	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("fileInfo"))
		if b == nil {
			return fmt.Errorf("read db error")
		}
		encoded, err := json.Marshal(fileInfo)
		if err != nil {
			return err
		}
		log.Debugf("Write DB: %s: %s", reqPath, string(encoded))
		return b.Put([]byte(reqPath), encoded)

	})
	if err != nil {
		// need to delete file
		deleteFileOnDisk(localPath)
		log.Error(err)
		json.NewEncoder(w).Encode(MakeErrInfo(ERR_UPDATE_DB))
		return
	}
	// success
	fileNum++
	fileInfo.DownloadPath = downloadPath
	responseInfo := UploadResponseInfo{
		ErrInfo: MakeErrInfo(ERR_OK),
		File:    *fileInfo,
	}
	json.NewEncoder(w).Encode(responseInfo)
	return
}
/*
Download file from server
Normal download:
curl -O http://localhost:50010/r/download_file/jianwang/ads.111
Gzip compress mode to download:
curl -H "Accept-Encoding: gzip"  http://localhost:50010/r/download_file/jianwang/ads.111 | gunzip >a.dmg
*/
func download(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log.Debugf("header: %v", r.Header)
	reqPath := ps.ByName("filepath")
	localPath := path.Join(svr.dataDir, reqPath)
	streamBytes, err := os.Open(localPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to open and read file : %v", err), 500)
		return
	}
	defer streamBytes.Close()
	httpgzip.ServeContent(w, r, localPath, time.Time{}, streamBytes)
}
/*
Backup database
curl  -o my.db http://localhost:50010/r/backup
 */
func backup(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	err := db.View(func(tx *bolt.Tx) error {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="my.db"`)
		w.Header().Set("Content-Length", strconv.Itoa(int(tx.Size())))
		_, err := tx.WriteTo(w)
		return err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
/*
Get file Server status
curl http://localhost:50010/r/status
{"Status":200,"Msg":"Online","IP":"172.24.48.137","FileNumber":5}
*/
func status(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log.Debugf("%s: %s, From: %s, Accept-Encoding: %s", r.Method, r.RequestURI, r.RemoteAddr, r.Header.Get("Accept-Encoding"))
	w.Header().Add("Content-Type", "application/json; charset=utf-8")

	hostOrIp, err := os.Hostname()
	if err != nil {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		for _, address := range addrs {
			// check if ip address is loopback
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					log.Println(ipnet.IP.String())
					hostOrIp = ipnet.IP.String()
				}
			}
		}
	}
	count := fileNum
	fileServer := FileServerInfo{
		ErrInfo:    MakeErrInfo(ERR_OK),
		ID:         hostOrIp + ":" + svr.port,
		FileNumber: count,
	}
	json.NewEncoder(w).Encode(fileServer)
}
/*
Get file Info or folder Info
file info: curl http://localhost:50010/r/info/data/danny/456.log
folder info: curl http://localhost:50010/r/info/\?isDir\=true\&recursion\=true\&suffix\=.log
*/
func info(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log.Debugf("%s: %s, From: %s", r.Method, r.RequestURI, r.RemoteAddr)
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	reqPath := ps.ByName("filepath")
	localPath := path.Join(svr.dataDir, reqPath)
	r.ParseForm()
	isDir := valuesGetDefault(r.Form, "isDir", "false")
	if strings.ToLower(isDir) != "false" && isDir != "0" {
		isDir = "true"
	}
	if isDir == "true" {
		recursion := valuesGetDefault(r.Form, "recursion", "true")
		if strings.ToLower(recursion) != "true" && recursion != "1" {
			recursion = "false"
		}
		suffix := valuesGetDefault(r.Form, "suffix", "")
		if recursion == "true" {
			files, err := WalkDir(localPath, suffix)
			if err != nil {
				log.Error(err)
			}
			json.NewEncoder(w).Encode(FileInfoResponse{
				ErrInfo: MakeErrInfo(ERR_OK),
				AllFile: files,
			})
			return
		} else {
			files, err := ListDir(localPath, suffix)
			if err != nil {
				log.Error(err)
			}
			json.NewEncoder(w).Encode(FileInfoResponse{
				ErrInfo: MakeErrInfo(ERR_OK),
				AllFile: files,
			})
			return
		}

	} else {
		var fileInfo *FileInfo
		err := db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("fileInfo"))
			if b == nil {
				return fmt.Errorf("read db error")
			}
			v := b.Get([]byte(reqPath))
			if v == nil {
				return nil
			}
			fileInfo = &FileInfo{}
			err := json.Unmarshal(v, fileInfo)
			return err
		})
		if err != nil {
			log.Error(err)
			json.NewEncoder(w).Encode(MakeErrInfo(ERR_READ_DB))
			return
		}
		if fileInfo == nil {
			json.NewEncoder(w).Encode(MakeErrInfo(ERR_FILE_NOT_IN_DB))
			return
		}
		if fileInfo.ExpiredTime.Before(time.Now()) {
			deleteFilesBothDiskAndDB(map[string]*FileInfo{reqPath: fileInfo})
		}
		localPath := path.Join(svr.dataDir, reqPath)
		if !checkFileIsExist(localPath) {
			json.NewEncoder(w).Encode(MakeErrInfo(ERR_FILE_NOT_EXIST))
			return
		}
		fileInfo.DownloadPath = "http://" + r.Host + "/r/download" + reqPath
		json.NewEncoder(w).Encode(FileInfoResponse{
			ErrInfo: MakeErrInfo(ERR_OK),
			File:    *fileInfo,
		})
		return
	}
}

/**
 * check if file exists, if exists, return true, else return false
 */
func checkFileIsExist(filename string) bool {
	var exist = true
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		exist = false
	}
	return exist
}
func clean(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log.Debugf("%s: %s, From: %s", r.Method, r.RequestURI, r.RemoteAddr)
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	var returnFiles struct {
		ErrInfo
		NumDeletedFiles int
		DeletedFiles    map[string]*FileInfo
	}
	returnFiles.DeletedFiles = getExpiredFiles()
	returnFiles.NumDeletedFiles = len(returnFiles.DeletedFiles)
	deleteFilesBothDiskAndDB(returnFiles.DeletedFiles)
	returnFiles.ErrInfo = MakeErrInfo(ERR_OK)
	json.NewEncoder(w).Encode(&returnFiles)
	return
}

func deleteExpiredFile() {
	ticker := time.NewTicker(time.Hour * 2)
	for range ticker.C {
		deleteFilesBothDiskAndDB(getExpiredFiles())
	}
}

func getExpiredFiles() (files map[string]*FileInfo) {
	files = make(map[string]*FileInfo)
	now := time.Now()
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("fileInfo"))
		if b == nil {
			log.Error("DB bucket fileInfo does not exist ")
			return nil
		}
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			temp := &FileInfo{}
			err := json.Unmarshal(v, temp)
			if err != nil {
				fmt.Errorf("error: %v", err)
				continue
			}
			if temp.ExpiredTime.Before(now) {
				files[string(k)] = temp
			}
		}
		return nil
	})
	return
}

func deleteFilesBothDiskAndDB(files map[string]*FileInfo) {
	dirs := make(map[string]bool)
	for f := range files {
		localPath := path.Join(svr.dataDir, f)
		log.Debugf("remove file: %s", localPath)
		if err := os.Remove(localPath); err != nil {
			log.Error(err)
		}
		err := db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("fileInfo"))
			if b == nil {
				return fmt.Errorf("read db error")
			}
			return b.Delete([]byte(f))
		})
		if err != nil {
			log.Error(err)
		}
		fileNum--
		for dir := path.Dir(localPath); dir != svr.dataDir && len(dir) > len(svr.dataDir); dir = path.Dir(dir) {
			dirs[dir] = true
		}
	}
	// if the directory is empty, remove it too
	dirsList := make([]string, 0, len(dirs))
	for k := range dirs {
		dirsList = append(dirsList, k)
	}
	sort.StringSlice(dirsList).Sort()
	for i := len(dirsList) - 1; i >= 0; i-- {
		f, err := os.Open(dirsList[i])
		if err != nil {
			log.Error(err)
		}
		fs, err2 := f.Readdirnames(1)
		if err2 == io.EOF && (fs == nil || len(fs) == 0) {
			f.Close()
			log.Debugf("remove dir: %s", dirsList[i])
			if err := os.Remove(dirsList[i]); err != nil {
				log.Error(err)
			}
			continue
		} else if err2 != nil {
			log.Error(err2)
		}
		f.Close()
	}
}

func deleteFileOnDisk(localPath string) {
	log.Debugf("remove file: %s", localPath)
	if err := os.Remove(localPath); err != nil {
		log.Error(err)
	}
	dirsList := make([]string, 0, 0)
	for dir := path.Dir(localPath); dir != svr.dataDir && len(dir) > len(svr.dataDir); dir = path.Dir(dir) {
		dirsList = append(dirsList, dir)
	}
	sort.StringSlice(dirsList).Sort()
	for i := len(dirsList) - 1; i >= 0; i-- {
		f, err := os.Open(dirsList[i])
		if err != nil {
			log.Error(err)
		}
		fs, err2 := f.Readdirnames(1)
		if err2 == io.EOF && (fs == nil || len(fs) == 0) {
			f.Close()
			log.Debugf("remove dir: %s", dirsList[i])
			if err := os.Remove(dirsList[i]); err != nil {
				log.Error(err)
			}
			continue
		} else if err2 != nil {
			log.Error(err2)
		}
		f.Close()
	}

}

func valuesGetDefault(values url.Values, key, defaultValue string) string {
	v := values.Get(key)
	if v == "" {
		return defaultValue
	} else {
		return v
	}
}

func InitDB() (*bolt.DB, error) {
	// use https://github.com/boltdb/bolt to save file info
	var dbErr error
	db, dbErr = bolt.Open("fileServer.db", 0600, nil)
	if dbErr != nil {
		return nil, fmt.Errorf("could not open db, %v", dbErr)
	}
	dbErr = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("fileInfo"))
		if err != nil {
			return fmt.Errorf("could not create root bucket: %v", err)
		}
		return nil
	})
	if dbErr != nil {
		return nil, fmt.Errorf("could not set up buckets, %v", dbErr)
	}
	return db, nil
}

func getFileNumInDB() int {
	count := 0
	db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte("fileInfo"))
		if b == nil {
			log.Info("DB bucket fileInfo does not exist ")
			return nil
		}
		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			count++
		}
		return nil
	})
	return count
}

//Get all the files in the specified directory, do not go to the next level directory search, you can match the suffix filter。
func ListDir(dirPth string, suffix string) (files []string, err error) {
	files = make([]string, 0, 10)
	dir, err := ioutil.ReadDir(dirPth)
	if err != nil {
		return nil, err
	}
	PthSep := string(os.PathSeparator)
	suffix = strings.ToUpper(suffix) //Case insensitive
	for _, fi := range dir {
		if fi.IsDir() { // Ignore directory
			continue
		}
		if strings.HasSuffix(strings.ToUpper(fi.Name()), suffix) {
			files = append(files, dirPth+PthSep+fi.Name())
		}
	}
	return files, nil
}

//Get all the files in the specified directory and all subdirectories, and match the suffix filter.
func WalkDir(dirPth, suffix string) (files []string, err error) {
	files = make([]string, 0, 30)
	suffix = strings.ToUpper(suffix)                                                     //Case insensitive
	err = filepath.Walk(dirPth, func(filename string, fi os.FileInfo, err error) error { //Reverse directory
		if err != nil {
			return err
		}
		if fi.IsDir() { // Ignore directory
			return nil
		}
		if strings.HasSuffix(strings.ToUpper(fi.Name()), suffix) {
			files = append(files, filename)
		}
		return nil
	})
	return files, err
}

func main() {
	flag.StringVar(&svr.logDir, "logDir", "logs", "dir to save all logs")
	flag.StringVar(&svr.dataDir, "dataDir", "data", "data directory")
	flag.StringVar(&svr.port, "port", "50010", "web api port")
	flag.StringVar(&svr.logLevel, "logLevel", "MORE", `log level: NORMAL, MORE, MUCH`)
	flag.Parse()
	var verbose log.VerboseLevel
	switch svr.logLevel {
	case "NORMAL":
		verbose = log.NORMAL
	case "MORE":
		verbose = log.MORE
	case "MUCH":
		verbose = log.MUCH
	default:
		log.Fatal("log_level only support: NORMAL, MORE, MUCH")
	}
	l := log.NewLoggerEx(1, verbose, "")
	if svr.logDir != "" {
		if err := os.MkdirAll(svr.logDir, os.ModePerm); err != nil {
			log.Fatal(err)
		}
		rw := log.NewRotateWriter(path.Join(svr.logDir, "repo.log"))
		defer rw.Close()
		l.SetOutput(rw)
	}
	log.SetStd(l)
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		time.Local = loc
	} else {
		log.Warn(err)
	}
	var err error
	svr.dataDir, err = filepath.Abs(svr.dataDir)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := InitDB(); err == nil {
		log.Println("DB init done")
		fileNum = getFileNumInDB()
	} else {
		log.Fatal(err)
		return
	}
	defer db.Close()
	go deleteExpiredFile()
	router := httprouter.New()
	router.ServeFiles("/r/list/*filepath", http.Dir(svr.dataDir))
	router.GET("/r/status", status)
	router.POST("/r/upload/*filepath", upload) // support http gzip compressed
	router.GET("/r/download/*filepath", download)
	router.GET("/r/info/*filepath", info)
	router.GET("/r/clean/", clean)
	router.GET("/r/backup", backup)
	log.Infof("run server on: %s", svr.port)
	http.ListenAndServe(":"+svr.port, router)

}
