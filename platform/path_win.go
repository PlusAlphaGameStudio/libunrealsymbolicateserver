//go:build windows

package platform

const addr2lineExePath = "~\\AppData\\Local\\Android\\Sdk\\ndk\\26.2.11394342\\toolchains\\llvm\\prebuilt\\windows-x86_64\\bin\\llvm-addr2line.exe"
const sevenZipExePath = "C:\\Program Files\\7-Zip\\7z.exe"

func GetAddr2lineExePath() string {
	return replaceHome(addr2lineExePath)
}

func GetSevenZipExePath() string {
	return replaceHome(sevenZipExePath)
}
