package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
)

/*
테스트 방법

curl -X POST http://localhost:8080/upload \
  -F "file=@~/LastUnhandledCrashStack.xml" \
  -H "Content-Type: multipart/form-data"

 */
func main() {
	router := gin.Default()
	// Set a lower memory limit for multipart forms (default is 32 MiB)
	router.MaxMultipartMemory = 8 << 20 // 8 MiB
	router.POST("/upload", func(c *gin.Context) {
		// single file
		file, _ := c.FormFile("file")
		log.Println(file.Filename)

		tempFilePath := path.Join(os.TempDir(), "libunrealsymbolicateserver.tmp")

		err := c.SaveUploadedFile(file, tempFilePath)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error());
			return
		}

		cmd := "BUILD_NUMBER=1424 cat \"" + tempFilePath + "\" | grep -E \"^ libUnreal \" | cut -d \"+\" -f 2 | ~/Library/Android/sdk/ndk/26.2.11394342/toolchains/llvm/prebuilt/darwin-x86_64/bin/llvm-addr2line -C -f -e ~/libUnreal.so"

		log.Println("Running: " + cmd)

		out, err := exec.Command("zsh","-c",cmd).Output()
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to execute command: %s", cmd)
			return
		}

		c.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!\n%s", file.Filename, string(out)))
	})

	_ = router.Run(":8080")
}
