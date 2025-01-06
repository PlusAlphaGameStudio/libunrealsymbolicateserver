package main

import (
	"bytes"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"io"
	"libunrealsymbolicateserver/platform"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

/*
테스트 방법

curl -X POST http://localhost:8080/upload -F "file=@~/LastUnhandledCrashStack.xml" -H "Content-Type: multipart/form-data"

curl.exe -X POST http://localhost:8080/upload -F "file=@LastUnhandledCrashStack.xml" -H "Content-Type: multipart/form-data"

 */
func main() {
	goDotErr := godotenv.Load()
	if goDotErr != nil {
		panic(errors.New("error loading .env file"))
	}

	router := gin.Default()
	// Set a lower memory limit for multipart forms (default is 32 MiB)
	router.MaxMultipartMemory = 8 << 20 // 8 MiB
	router.POST("/", func(c *gin.Context) {
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

		var buildNumber int64
		var addrLines []string
		for _, line := range strings.Split(string(uploadBytes), "\n") {
			if strings.HasPrefix(line, " libUnreal ") {
				addrLines = append(addrLines, strings.Split(line, " + ")[1])
			}

			if strings.HasPrefix(line, "<RipperBuildNumber>") {
				// '1234 Dev' 또는 '4567 Shi' 등의 값이다. 숫자만 뽑아 온다.
				buildNumberStr := strings.Split(line[len("<RipperBuildNumber>"):], " ")[0]
				if buildNumber, err = strconv.ParseInt(buildNumberStr, 10, 32); err != nil {
					c.String(http.StatusInternalServerError, err.Error())
					return
				}
			}
		}

		var out []byte
		libUnrealPath := strings.ReplaceAll(os.Getenv("LIB_UNREAL_PATH"), "{BuildNumber}", strconv.FormatInt(buildNumber, 10))
		subProcess := exec.Command(platform.GetAddr2lineExePath(), "-C", "-f", "-e", libUnrealPath)
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
	router.StaticFS("/", http.Dir("static"))

	_ = router.Run(os.Getenv("SYMBOLICATE_SERVER_ADDR"))
}

type Frame struct {
	Function string
	Args string
	File string
}

type SymbolicateResult struct {
	Frames []Frame
}