package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"libunrealsymbolicateserver/platform"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

/*
테스트 방법

curl -X POST http://localhost:8080/upload -F "file=@~/LastUnhandledCrashStack.xml" -H "Content-Type: multipart/form-data"

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


		if err := c.SaveUploadedFile(file, tempFile.Name()); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		time.Sleep(1 * time.Second)

		cmd := fmt.Sprintf(platform.Cmd, tempFile.Name(), platform.Addr2lineExePath)

		log.Println("Running: " + cmd)

		var out []byte
		if runtime.GOOS == "windows" {
			out, err = exec.Command("powershell","-nologo", "-noprofile", cmd).Output()
		} else {
			out, err = exec.Command("zsh","-c", cmd).Output()
		}

		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to execute command: %s", cmd)
			return
		}

		outStr := string(out)
		outLines := strings.Split(outStr, "\n")

		combinedStr := ""
		for i, line := range outLines {
			combinedStr += line
			if i != 0 && i % 2 == 0 {
				combinedStr += "\n"
			} else {
				combinedStr += "    "
			}
		}

		c.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!\n%s", file.Filename, combinedStr))
	})

	_ = router.Run(":8080")
}
