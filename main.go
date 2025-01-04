package main

import (
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"libunrealsymbolicateserver/platform"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

/*
테스트 방법

curl -X POST http://localhost:8080/upload -F "file=@~/LastUnhandledCrashStack.xml" -H "Content-Type: multipart/form-data"

curl.exe -X POST http://localhost:8080/upload -F "file=@LastUnhandledCrashStack.xml" -H "Content-Type: multipart/form-data"

 */
func main() {
	router := gin.Default()
	// Set a lower memory limit for multipart forms (default is 32 MiB)
	router.MaxMultipartMemory = 8 << 20 // 8 MiB
	router.POST("/upload", func(c *gin.Context) {
		// single file
		file, _ := c.FormFile("file")
		log.Println(file.Filename)

		//goland:noinspection SpellCheckingInspection
		tempFile, err := os.CreateTemp(os.TempDir(), "libunrealsymbolicateserver-*.tmp")
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer func(tempFile *os.File) {
			_ = tempFile.Close()
		}(tempFile)

		if err := c.SaveUploadedFile(file, tempFile.Name()); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		uploadBytes, err := os.ReadFile(tempFile.Name())
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		var addrLines []string
		for _, line := range strings.Split(string(uploadBytes), "\n") {
			if strings.HasPrefix(line, " libUnreal ") {
				addrLines = append(addrLines, strings.Split(line, " + ")[1])
			}
		}

		var out []byte
		subProcess := exec.Command(platform.Addr2lineExePath, "-C", "-f", "-e", platform.LibUnrealPath)
		stdinPipe, err := subProcess.StdinPipe()
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer func(stdinPipe io.WriteCloser) {
			_ = stdinPipe.Close()
		}(stdinPipe)
		outBytes := bytes.NewBuffer([]byte{})
		subProcess.Stdout = outBytes
		subProcess.Stderr = os.Stderr
		if err := subProcess.Start(); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		for _, line := range addrLines {
			if _, err := io.WriteString(stdinPipe, line + "\n"); err != nil {
				c.String(http.StatusInternalServerError, err.Error())
				return
			}
		}

		if err := stdinPipe.Close(); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		if err := subProcess.Wait(); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		out = outBytes.Bytes()

		outStr := string(out)
		outLines := strings.Split(outStr, "\n")

		combinedStr := ""
		for i := range outLines {
			if i % 2 != 0 {
				continue
			}

			if len(outLines) <= i + 1 {
				break
			}

			combinedStr += outLines[i] + "    " + outLines[i + 1] + "\n"
		}

		c.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!\n%s", file.Filename, combinedStr))
	})

	_ = router.Run(":8080")
}
