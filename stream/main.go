package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	_ "github.com/paypal/katbox/stream/docs"
	httpSwagger "github.com/swaggo/http-swagger"
)

type StreamData struct {
	Data   string `json:"data"`
	Offset int64  `json:"offset"`
}

type FileInformation struct {
	Mode  string `json:"mode"`
	Nlink string `json:"nlink"`
	UID   string `json:"uid"`
	GID   string `json:"gid"`
	Size  string `json:"size"`
	Mtime int64  `json:"mtime"`
	Path  string `json:"path"`
}

type Result struct {
	Data []FileInformation `json:"data`
}

type Error struct {
	Error string `json:"error"`
}

// Download a file from sandbox filesystem with given offset and length godoc
// @Summary Download a file from Filesystem
// @Description Download any file from sandbox logs filesystem
// @Accept  json
// @Produce octet-stream
// @Success 200 {object} main.StreamData
// @Router /files/download [get]
// @Param path query string true "Path"
func downloadhandler(w http.ResponseWriter, r *http.Request) {
	//Get query params
	params := r.URL.Query()
	path := params["path"]

	if len(path) == 0 {
		log.Println("path not found in URL")
		error := &Error{
			Error: "path not found in URL"}
		retObj, err := json.Marshal(error)
		if err != nil {
			log.Print(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(retObj)
		return
	}

	file, err := os.Open(path[0])
	if err != nil {
		log.Print(err)
	}
	filename := filepath.Base(path[0])

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	io.Copy(w, file)
	file.Close()

}

// read a file from sandbox filesystem with given offset and length godoc
// @Summary Read a file from Filesystem
// @Description Reads any file from sandbox logs filesystem and serves as a json object
// @Accept  json
// @Success 200 {object} main.StreamData
// @Router /files/read [get]
// @Param path query string true "Path"
// @Param offset query int true "Offset"
// @Param length query int true "Length"
// @Param jsonp query string true "jsonp"
func readHandler(w http.ResponseWriter, r *http.Request) {
	// Get query path params
	params := r.URL.Query()
	path := params["path"]

	//check if path exists in the query params if not send error response
	if len(path) == 0 {
		log.Println("path not found in URL")
		error := &Error{
			Error: "path not found in URL"}
		retObj, err := json.Marshal(error)
		if err != nil {
			log.Print(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(retObj)
		return
	}

	var err error
	//check if offset exists in the query params if not send error response
	if len(params["offset"]) == 0 {
		error := &Error{
			Error: "offset not found in URL"}
		retObj, err := json.Marshal(error)
		if err != nil {
			log.Print(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(retObj)
		return
	}
	offset, err := strconv.ParseInt(params["offset"][0], 10, 64)
	if err != nil {
		log.Print(err)
	}

	//check if length exists in the query params if not send error response
	if len(params["length"]) == 0 {
		error := &Error{
			Error: "length not found in URL"}
		retObj, err := json.Marshal(error)
		if err != nil {
			log.Print(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(retObj)
		return
	}
	length, err := strconv.ParseInt(params["length"][0], 10, 64)
	if err != nil {
		log.Print(err)
	}

	size := getSize(path[0])

	// if offset is invalid set it to size of the file
	if offset == -1 {
		offset = size
	}

	//set length if it is invalid
	if length == -1 {
		length = size - offset
	}

	// Cap the read length at 16 pages.
	length = int64(math.Min(float64(length), float64(os.Getpagesize()*16)))

	// return empty data when offset is greater than size of the file
	if offset >= size {
		streamData := &StreamData{
			Data:   "",
			Offset: size}

		respObj, err := json.Marshal(streamData)
		if err != nil {
			fmt.Fprintf(w, "json encode error")
			return
		}

		callbackName := r.URL.Query().Get("jsonp")
		if callbackName == "" {
			fmt.Fprintf(w, "Please give callback name in query string")
			return
		}

		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		fmt.Fprintf(w, "%s(%s);", callbackName, respObj)

		return
	}

	file, err := os.Open(path[0])
	if err != nil {
		log.Print(err)
	}

	defer file.Close()

	s := io.NewSectionReader(file, 0, size)

	buf2 := make([]byte, length)
	_, err = s.ReadAt(buf2, offset)
	if err != nil {
		log.Print(err)
	}

	staremData := &StreamData{
		Data:   string(buf2),
		Offset: offset}

	respObj, err := json.Marshal(staremData)
	if err != nil {
		fmt.Fprintf(w, "json encode error")
		return
	}

	callbackName := r.URL.Query().Get("jsonp")
	if callbackName == "" {
		fmt.Fprintf(w, "Please give callback name in query string")
		return
	}

	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	fmt.Fprintf(w, "%s(%s);", callbackName, respObj)

}

// browse sandbox filesystem godoc
// @Summary Browse Filesystem
// @Description serves sandbox logs filesystem as a json object
// @Accept  json
// @Produce  json
// @Success 200 {object} main.Result
// @Router /files/browse [get]
// @Param path query string true "Path"
func browseHandler(w http.ResponseWriter, r *http.Request) {
	//Get query Params
	params := r.URL.Query()
	path := params["path"]

	//check if path exists in the query params if not send error response
	if len(path) == 0 {
		log.Println("path not found in URL")
		error := &Error{
			Error: "path not found in URL"}
		retObj, err := json.Marshal(error)
		if err != nil {
			log.Print(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(retObj)
		return
	}

	// walk through the directory structure and build fileInformation struct and append to result array
	result := []FileInformation{}
	//test := strings.Split(path[0], "/")[len(strings.Split(path[0], "/"))-1]
	//fmt.Print(test, "   ")
	//fmt.Print(test, "   ")
	err := filepath.WalkDir(path[0],
		func(filePath string, dirEntry os.DirEntry, err error) error {
			if len(strings.Split(filePath, "/")) < len(strings.Split(path[0], "/"))+2 {
				if err != nil {
					return err
				}
				fileInfo, err := dirEntry.Info()

				fileStat, err := os.Stat(filePath)
				if err != nil {
					return err
				}
				// retrieve file ownership information and number of hardlinks to the file
				var nlink, uid, gid uint64
				if sys := fileStat.Sys(); sys != nil {
					if stat, ok := sys.(*syscall.Stat_t); ok {
						nlink = uint64(stat.Nlink)
						uid = uint64(stat.Uid)
						gid = uint64(stat.Gid)
					}
				}
				usr, err := user.LookupId(strconv.FormatUint(uid, 10))
				group, err := user.LookupGroupId(strconv.FormatUint(gid, 10))
				if err != nil {
					return err
				}

				jsonData := FileInformation{
					Mode:  fileInfo.Mode().String(),
					Nlink: string(strconv.FormatUint(nlink, 10)),
					UID:   usr.Username,
					GID:   group.Name,
					Size:  strconv.Itoa(int(fileInfo.Size())),
					Mtime: fileInfo.ModTime().Unix(),
					Path:  filePath,
				}
				if err == nil {
					result = append(result, jsonData)
				}
			}

			return nil
		})
	if err != nil {
		log.Println(err)
	}

	resultdata := Result{result}
	c, err := json.Marshal(resultdata)

	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(c)
	}

}

//Get the size of a file
func getSize(p string) int64 {
	if stat, err := os.Stat(p); err == nil {
		return stat.Size()
	}
	return 0
}

// @title k8s Sandbox Go Restful API with Swagger
// @version 1.0
// @description Rest API doc for sandbox API's
// @contact.name Revanth Chandra
// @host localhost:8080
// @BasePath /
func main() {
	http.HandleFunc("/files/read", readHandler)
	http.HandleFunc("/files/browse", browseHandler)
	http.HandleFunc("/files/download", downloadhandler)
	http.HandleFunc("/swagger/", httpSwagger.WrapHandler)
	http.ListenAndServe(":5051", nil)
}
