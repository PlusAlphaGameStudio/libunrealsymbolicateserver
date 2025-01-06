package main

import (
	"bytes"
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
		subProcess := exec.Command(platform.GetAddr2lineExePath(), "-C", "-f", "-e", platform.GetLibUnrealPath())
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

		var data SymbolicateResult

		combinedStr := ""
		for i := range outLines {
			if i % 2 != 0 {
				continue
			}

			if len(outLines) <= i + 1 {
				break
			}

			combinedStr += outLines[i] + "    " + outLines[i + 1] + "\n"

			var function string
			var args string
			argIndex := strings.Index(outLines[i], "(")
			if argIndex >= 0 {
				function = outLines[i][0:argIndex]
				args = outLines[i][argIndex:]
			} else {
				function = outLines[i]
			}

			data.Frames = append(data.Frames, Frame{
				Function: function,
				Args: args,
				File:     outLines[i + 1],
			})
		}

		c.HTML(http.StatusOK, "result.tmpl", data)
	})

	router.LoadHTMLGlob("templates/*")
	router.StaticFS("/upload", http.Dir("static"))

	_ = router.Run(":8080")
}

type Frame struct {
	Function string
	Args string
	File string
}

type SymbolicateResult struct {
	Frames []Frame
}