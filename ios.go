package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"libunrealsymbolicateserver/platform"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type FGenericCrashContext struct {
	XMLName            xml.Name           `xml:"FGenericCrashContext"`
	RuntimeProperties  RuntimeProperties  `xml:"RuntimeProperties"`
	PlatformProperties PlatformProperties `xml:"PlatformProperties"`
	EngineData         EngineData         `xml:"EngineData"`
	GameData           GameData           `xml:"GameData"`
}

type RuntimeProperties struct {
	CrashVersion                       int    `xml:"CrashVersion"`
	ExecutionGuid                      string `xml:"ExecutionGuid"`
	CrashGUID                          string `xml:"CrashGUID"`
	IsEnsure                           bool   `xml:"IsEnsure"`
	IsStall                            bool   `xml:"IsStall"`
	IsAssert                           bool   `xml:"IsAssert"`
	CrashType                          string `xml:"CrashType"`
	ErrorMessage                       string `xml:"ErrorMessage"`
	CrashReporterMessage               string `xml:"CrashReporterMessage"`
	ProcessId                          int    `xml:"ProcessId"`
	SecondsSinceStart                  int    `xml:"SecondsSinceStart"`
	IsInternalBuild                    bool   `xml:"IsInternalBuild"`
	IsPerforceBuild                    bool   `xml:"IsPerforceBuild"`
	IsWithDebugInfo                    bool   `xml:"IsWithDebugInfo"`
	IsSourceDistribution               bool   `xml:"IsSourceDistribution"`
	GameName                           string `xml:"GameName"`
	ExecutableName                     string `xml:"ExecutableName"`
	BuildConfiguration                 string `xml:"BuildConfiguration"`
	GameSessionID                      string `xml:"GameSessionID"`
	PlatformName                       string `xml:"PlatformName"`
	PlatformFullName                   string `xml:"PlatformFullName"`
	PlatformNameIni                    string `xml:"PlatformNameIni"`
	EngineMode                         string `xml:"EngineMode"`
	EngineModeEx                       string `xml:"EngineModeEx"`
	DeploymentName                     string `xml:"DeploymentName"`
	EngineVersion                      string `xml:"EngineVersion"`
	EngineCompatibleVersion            string `xml:"EngineCompatibleVersion"`
	CommandLine                        string `xml:"CommandLine"`
	LanguageLCID                       int    `xml:"LanguageLCID"`
	AppDefaultLocale                   string `xml:"AppDefaultLocale"`
	BuildVersion                       string `xml:"BuildVersion"`
	Symbols                            string `xml:"Symbols"`
	IsUERelease                        bool   `xml:"IsUERelease"`
	IsRequestingExit                   bool   `xml:"IsRequestingExit"`
	UserName                           string `xml:"UserName"`
	BaseDir                            string `xml:"BaseDir"`
	RootDir                            string `xml:"RootDir"`
	MachineId                          string `xml:"MachineId"`
	LoginId                            string `xml:"LoginId"`
	EpicAccountId                      string `xml:"EpicAccountId"`
	SourceContext                      string `xml:"SourceContext"`
	UserDescription                    string `xml:"UserDescription"`
	UserActivityHint                   string `xml:"UserActivityHint"`
	CrashDumpMode                      int    `xml:"CrashDumpMode"`
	GameStateName                      string `xml:"GameStateName"`
	NumberOfCores                      int    `xml:"Misc.NumberOfCores"`
	NumberOfCoresIncludingHyperthreads int    `xml:"Misc.NumberOfCoresIncludingHyperthreads"`
	Is64bitOperatingSystem             int    `xml:"Misc.Is64bitOperatingSystem"`
	CPUVendor                          string `xml:"Misc.CPUVendor"`
	CPUBrand                           string `xml:"Misc.CPUBrand"`
	PrimaryGPUBrand                    string `xml:"Misc.PrimaryGPUBrand"`
	OSVersionMajor                     string `xml:"Misc.OSVersionMajor"`
	OSVersionMinor                     string `xml:"Misc.OSVersionMinor"`
	AnticheatProvider                  string `xml:"Misc.AnticheatProvider"`
	TotalPhysical                      int64  `xml:"MemoryStats.TotalPhysical"`
	TotalVirtual                       int64  `xml:"MemoryStats.TotalVirtual"`
	PageSize                           int    `xml:"MemoryStats.PageSize"`
	TotalPhysicalGB                    int    `xml:"MemoryStats.TotalPhysicalGB"`
	AvailablePhysical                  int64  `xml:"MemoryStats.AvailablePhysical"`
	AvailableVirtual                   int64  `xml:"MemoryStats.AvailableVirtual"`
	UsedPhysical                       int64  `xml:"MemoryStats.UsedPhysical"`
	PeakUsedPhysical                   int64  `xml:"MemoryStats.PeakUsedPhysical"`
	UsedVirtual                        int64  `xml:"MemoryStats.UsedVirtual"`
	PeakUsedVirtual                    int64  `xml:"MemoryStats.PeakUsedVirtual"`
	IsOOM                              int    `xml:"MemoryStats.bIsOOM"`
	OOMAllocationSize                  int64  `xml:"MemoryStats.OOMAllocationSize"`
	OOMAllocationAlignment             int    `xml:"MemoryStats.OOMAllocationAlignment"`
	NumMinidumpFramesToIgnore          int    `xml:"NumMinidumpFramesToIgnore"`
	CallStack                          string `xml:"CallStack"`
	PCallStack                         string `xml:"PCallStack"`
	PCallStackHash                     string `xml:"PCallStackHash"`
}

type PlatformProperties struct {
	AdditionalSystemSymbolsVersion string `xml:"AdditionalSystemSymbolsVersion"`
	PlatformCallbackResult         int    `xml:"PlatformCallbackResult"`
	CrashTrigger                   int    `xml:"CrashTrigger"`
}

type EngineData struct {
	MatchingDPStatus  string `xml:"MatchingDPStatus"`
	RHIName           string `xml:"RHI.RHIName"`
	AdapterName       string `xml:"RHI.AdapterName"`
	FeatureLevel      string `xml:"RHI.FeatureLevel"`
	GPUVendor         string `xml:"RHI.GPUVendor"`
	DeviceId          string `xml:"RHI.DeviceId"`
	DeviceProfileName string `xml:"DeviceProfile.Name"`
}

type GameData struct {
	RipperBuildNumber string `xml:"RipperBuildNumber"`
	RipperBuildDate   string `xml:"RipperBuildDate"`
	RipperVersion     string `xml:"RipperVersion"`
	LibUnrealBuildID  string `xml:"LibUnrealBuildID"`
}

type CallStackEntry struct {
	ModuleName  string
	BaseAddress int64
	Offset      int64
}

type CrashContextResult struct {
	RipperBuildNumber int
	LibUnrealBuildID  string
	CallStackEntries  []CallStackEntry
}

func unzipUsing7zAndFindRipper(zipPath string) (string, error) {
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

	// 압축 해제된 파일들 중 'Ripper' 이름을 가진 파일을 재귀적으로 찾기
	var ripperPath string
	err = filepath.Walk(tmpDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "Ripper" {
			ripperPath = filePath
			return filepath.SkipDir // 찾았으므로 더 이상 검색하지 않음
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	if ripperPath == "" {
		return "", errors.New("'Ripper' file not found in extracted files")
	}

	return ripperPath, nil
}

func symbolicateIos(uploadBytes []byte) ([]byte, error) {
	crashResult, err := parseFGenericCrashContext(uploadBytes)
	if err != nil {
		return nil, err
	}

	libUnrealPath := strings.ReplaceAll(os.Getenv("LIB_UNREAL_PATH"), "{BuildNumber}", strconv.Itoa(crashResult.RipperBuildNumber))

	libZipPath := recursivelyFindDsymZipPathByBuildId(libUnrealPath, crashResult.LibUnrealBuildID)

	ripperBinPath, err := unzipUsing7zAndFindRipper(libZipPath)
	if err != nil {
		return nil, err
	}

	log.Println("libUnreal path found: " + ripperBinPath)

	subProcess := exec.Command(platform.GetXCRunExePath(), "atos", "-o", ripperBinPath, "-arch", "arm64", "-l")
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

	for _, entry := range crashResult.CallStackEntries {
		var inputLine string
		if entry.ModuleName == "Ripper" {
			address := entry.BaseAddress + entry.Offset
			inputLine = strconv.FormatInt(address, 16)
		} else {
			inputLine = "?"
		}

		if _, err := io.WriteString(stdinPipe, inputLine+"\n"); err != nil {
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

func parseFGenericCrashContext(xmlData []byte) (*CrashContextResult, error) {
	var crashContext FGenericCrashContext
	if err := xml.Unmarshal(xmlData, &crashContext); err != nil {
		return nil, err
	}

	var callStackEntries []CallStackEntry
	pCallStack := crashContext.RuntimeProperties.PCallStack

	lines := strings.Split(strings.TrimSpace(pCallStack), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 4 && parts[2] == "+" {
			baseAddr, err := strconv.ParseInt(parts[1], 16, 64)
			if err != nil {
				continue
			}

			offset, err := strconv.ParseInt(parts[3], 16, 64)
			if err != nil {
				continue
			}

			entry := CallStackEntry{
				ModuleName:  parts[0],
				BaseAddress: baseAddr,
				Offset:      offset,
			}
			callStackEntries = append(callStackEntries, entry)
		}
	}

	buildNumber := 0
	buildNumberStr := strings.Fields(crashContext.GameData.RipperBuildNumber)[0]
	if parsedBuildNumber, err := strconv.Atoi(buildNumberStr); err == nil {
		buildNumber = parsedBuildNumber
	}

	result := &CrashContextResult{
		RipperBuildNumber: buildNumber,
		LibUnrealBuildID:  crashContext.GameData.LibUnrealBuildID,
		CallStackEntries:  callStackEntries,
	}

	return result, nil
}

func recursivelyFindDsymZipPathByBuildId(basePath string, buildId string) string {
	files, err := glob(basePath, ".7z")
	if err != nil {
		panic(err)
	}

	for _, g := range files {
		// 찾아야 되는 파일명의 예시는...
		// Ripper-dSYM-C6F57CCE-A15E-30D9-A154-50BD91207CB8.7z
		if strings.Contains(g, "Ripper-dSYM-"+buildId+".7z") {
			return g
		}
	}

	return ""
}
