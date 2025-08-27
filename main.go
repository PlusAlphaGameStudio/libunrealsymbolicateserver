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
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

/*
테스트 방법

.env 파일 이용해서...

SYMBOLICATE_SERVER_ADDR 환경 변수를 설정한다. (e.g. :8123)
LIB_UNREAL_PATH 환경 변수를 설정한다. (e.g. /unreallib/path)

[macOS]

curl -X POST http://localhost:8080/upload -F "file=@~/LastUnhandledCrashStack.xml" -H "Content-Type: multipart/form-data"

[Windows]

curl.exe -X POST http://localhost:8080/upload -F "file=@LastUnhandledCrashStack.xml" -H "Content-Type: multipart/form-data"
*/
func main() {
	goDotErr := godotenv.Load()
	if goDotErr != nil {
		panic(errors.New("error loading .env file"))
	}

	platform.ExecuteBatchSelfTests(selfTestSingle)

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

		var outBytes []byte
		if outBytes, err = symbolicate(uploadBytes); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		outStr := string(outBytes)
		outLines := strings.Split(outStr, "\n")

		var data SymbolicateResult

		combinedStr := ""
		for i := range outLines {
			if i%2 != 0 {
				continue
			}

			if len(outLines) <= i+1 {
				break
			}

			combinedStr += outLines[i] + "    " + outLines[i+1] + "\n"

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
				Args:     args,
				File:     outLines[i+1],
			})
		}

		c.HTML(http.StatusOK, "result.tmpl", data)
	})

	router.LoadHTMLGlob("templates/*")
	router.StaticFS("/", http.Dir("static"))

	_ = router.Run(os.Getenv("SYMBOLICATE_SERVER_ADDR"))
}

func symbolicate(uploadBytes []byte) ([]byte, error) {
	// Check if it's iOS platform
	if strings.Contains(string(uploadBytes), "<PlatformName>IOS</PlatformName>") {
		return symbolicateIos(uploadBytes)
	}

	// Default to Android symbolication
	return symbolicateAndroid(uploadBytes)
}

func symbolicateAndroid(uploadBytes []byte) ([]byte, error) {
	var buildNumber int64
	var addrLines []string
	var libUnrealBuildID string

	started := false
	for _, line := range strings.Split(string(uploadBytes), "\n") {
		added := false
		if strings.HasPrefix(line, " libUnreal ") {
			addrLines = append(addrLines, strings.Split(line, " + ")[1])
		} else if strings.Contains(line, "/libUnreal.so") {
			lineTokens := strings.Fields(strings.Trim(line, " "))
			if len(lineTokens) >= 4 {
				if strings.HasSuffix(lineTokens[3], "/libUnreal.so") {
					addrLines = append(addrLines, lineTokens[2])
					added = true
					started = true
				}
			}
		} else if strings.Contains(line, "libUnreal.so(") {
			lineTokens := strings.Fields(strings.Trim(line, " "))
			if len(lineTokens) >= 2 {
				iBeg := strings.Index(lineTokens[1], "(")
				iEnd := strings.Index(lineTokens[1], ")")
				if iBeg >= 0 && iEnd >= 0 {
					addrLines = append(addrLines, lineTokens[1][iBeg+1:iEnd])
				}
			}
		}

		// 주소 한 뭉텅이가 다 처리됐으면 그 다음은 다 무시한다.
		if added == false && started == true {
			break
		}

		if strings.HasPrefix(line, "<RipperBuildNumber>") {
			// '1234 Dev' 또는 '4567 Shi' 등의 값이다. 숫자만 뽑아 온다.
			buildNumberStr := strings.Split(line[len("<RipperBuildNumber>"):], " ")[0]
			var err error
			if buildNumber, err = strconv.ParseInt(buildNumberStr, 10, 32); err != nil {
				return nil, err
			}
		}

		if strings.HasPrefix(line, "<LibUnrealBuildID>") {
			// <LibUnrealBuildID>xxxxx</LibUnrealBuildID> 중 xxxxx 부분 뽑아온다.
			line = strings.TrimSpace(line)
			libUnrealBuildID = line[len("<LibUnrealBuildID>") : len(line)-len("</LibUnrealBuildID>")]

		}
	}

	log.Println("=== Filtered address lines begin ===")
	for _, l := range addrLines {
		log.Println(l)
	}
	log.Println("=== Filtered address lines end ===")

	// 혹시 tombstone이면 libUnrealBuildID 아직 비어있을 것이다.
	// 채우기 시도한다.
	if len(libUnrealBuildID) == 0 {
		libUnrealBuildID = findBuildIdFromTombstone(uploadBytes)
	}

	// 그래도 안채워져있으면 가장 단순한 형태인 LastCrashStack.txt 일 것이다.
	// 이것도 아니면 말고...
	if len(libUnrealBuildID) == 0 {
		libUnrealBuildID = findBuildIdFromTxt(uploadBytes)
	}

	var libUnrealPath string
	if buildNumber > 0 {
		libUnrealPath = strings.ReplaceAll(os.Getenv("LIB_UNREAL_PATH"), "{BuildNumber}", strconv.FormatInt(buildNumber, 10))
	} else {
		libUnrealPath = strings.ReplaceAll(os.Getenv("LIB_UNREAL_PATH"), "{BuildNumber}", "")
	}

	libZipPath := recursivelyFindLibZipPathByBuildId(libUnrealPath, libUnrealBuildID)

	extractedLibPath, err := unzipUsing7z(libZipPath)
	if err != nil {
		return nil, err
	}

	log.Println("libUnreal path found: " + extractedLibPath)

	subProcess := exec.Command(platform.GetAddr2lineExePath(), "-C", "-f", "-e", extractedLibPath)
	stdinPipe, err := subProcess.StdinPipe()
	if err != nil {
		return nil, err
	}
	defer func(stdinPipe io.WriteCloser) {
		_ = stdinPipe.Close()
	}(stdinPipe)
	outBytesBuffer := bytes.NewBuffer([]byte{})
	subProcess.Stdout = outBytesBuffer
	subProcess.Stderr = os.Stderr
	if err := subProcess.Start(); err != nil {
		return nil, err
	}

	for _, line := range addrLines {
		if _, err := io.WriteString(stdinPipe, line+"\n"); err != nil {
			return nil, err
		}
	}

	if err := stdinPipe.Close(); err != nil {
		return nil, err
	}

	if err := subProcess.Wait(); err != nil {
		return nil, err
	}

	outBytes := outBytesBuffer.Bytes()

	return outBytes, nil
}

func findBuildIdFromTxt(uploadBytes []byte) string {
	for _, line := range strings.Split(string(uploadBytes), "\n") {
		if strings.HasPrefix(line, "Build ID: ") {
			return strings.TrimSpace(line[len("Build ID: "):])
		}
	}
	return ""
}

func selfTestSingle(samplePath string) {
	sampleBytes, err := os.ReadFile(samplePath)
	if err != nil {
		log.Fatalf(err.Error())
		return
	}

	resultBytes, err := symbolicate(sampleBytes)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("============= " + samplePath + " =============")
	log.Println(string(sampleBytes))
	log.Println("============= " + samplePath + " (Result) =============")
	log.Println(string(resultBytes))
}

func unzipUsing7z(zipPath string) (string, error) {
	if len(zipPath) == 0 {
		return "", errors.New("empty zipPath 1")
	}

	tmpDir := os.TempDir()
	subProcess := exec.Command(platform.GetSevenZipExePath(), "e", "-y", "-o"+tmpDir, zipPath)
	subProcess.Stdout = os.Stdout

	log.Println("Running external command: " + subProcess.String())
	err := subProcess.Start()
	if err != nil {
		return "", err
	}
	err = subProcess.Wait()
	if err != nil {
		return "", err
	}

	return path.Join(tmpDir, "Ripper-arm64.so"), nil
}

func findBuildIdFromTombstone(uploadBytes []byte) string {
	// 아래와 같은 행에서 BuildId 값을 뽑아낸다.
	// 이 경우에는 f7dda47241ed05630db4fb766057f679a7753099를 뽑아낸다.

	// tombstone인 경우에는 다음과 같다.
	// "#08 pc 00000000103a0b24  /data/app/~~36X5uXJu4u54AQD_SFtvVQ==/top.plusalpha.ripper-9D_AnEX8sOyxHH-off443w==/lib/arm64/libUnreal.so (BuildId: f7dda47241ed05630db4fb766057f679a7753099)

	for _, line := range strings.Split(string(uploadBytes), "\n") {
		if strings.Contains(line, "/libUnreal.so ") && strings.Contains(line, "(BuildId: ") {

			start := strings.Index(line, "(BuildId: ")
			end := len(line) - 1

			return line[start+len("(BuildId: ") : end]
		}
	}

	return ""
}

func glob(dir string, ext string) ([]string, error) {

	var files []string
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if filepath.Ext(path) == ext {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

// basePath 및 하위 디렉토리를 재귀적으로 탐색해서 buildId에 해당하는
// 심볼 파일의 압축 버전 경로를 반환한다.
func recursivelyFindLibZipPathByBuildId(basePath string, buildId string) string {
	suffix := "-" + buildId + ".7z"
	log.Printf("Recursively finding a file from '" + basePath + "' with suffix '" + suffix + "'...")

	files, err := glob(basePath, ".7z")
	if err != nil {
		panic(err)
	}

	for _, g := range files {
		// 찾아야 되는 파일명의 예시는...
		// Ripper-arm64-f7dda47241ed05630db4fb766057f679a7753099.7z
		if strings.HasSuffix(g, suffix) {
			return g
		}
	}

	return ""
}

type Frame struct {
	Function string
	Args     string
	File     string
}

type SymbolicateResult struct {
	Frames []Frame
}
