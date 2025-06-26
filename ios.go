package main

import (
	"bytes"
	"encoding/xml"
	"io"
	"libunrealsymbolicateserver/platform"
	"log"
	"os"
	"os/exec"
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

func symbolicateIos(uploadBytes []byte) ([]byte, error) {
	var buildNumber int64
	var addrLines []string
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
		}

		// 주소 한 뭉텅이가 다 처리됐으면 그 다음은 다 무시한다.
		if added == false && started == true {
			break
		}

		if strings.HasPrefix(line, "<RipperBuildNumber>") {
			// '1234 Dev' 또는 '4567 Shi' 등의 값이다. 숫자만 뽑아 온다.
			buildNumberStr := strings.Split(line[len("<RipperBuildNumber>"):], " ")[0]
			if _, err := strconv.ParseInt(buildNumberStr, 10, 32); err != nil {
				return nil, err
			}
		}
	}

	log.Println("=== Filtered address lines begin ===")
	for _, l := range addrLines {
		log.Println(l)
	}
	log.Println("=== Filtered address lines end ===")

	buildId := findBuildId(uploadBytes)

	libUnrealPath := strings.ReplaceAll(os.Getenv("LIB_UNREAL_PATH"), "{BuildNumber}", strconv.FormatInt(buildNumber, 10))

	libZipPath := recursivelyFindLibZipPathByBuildId(libUnrealPath, buildId)

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
