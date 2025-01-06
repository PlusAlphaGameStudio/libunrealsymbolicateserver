//go:build windows

package platform

const Addr2lineExePath = "~\\AppData\\Local\\Android\\Sdk\\ndk\\26.2.11394342\\toolchains\\llvm\\prebuilt\\windows-x86_64\\bin\\llvm-addr2line.exe"

func GetAddr2lineExePath() string {
	return replaceHome(addr2lineExePath)
}
