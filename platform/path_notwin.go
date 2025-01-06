//go:build !windows

package platform

const addr2lineExePath = "~/Library/Android/sdk/ndk/26.2.11394342/toolchains/llvm/prebuilt/darwin-x86_64/bin/llvm-addr2line"

func GetAddr2lineExePath() string {
	return replaceHome(addr2lineExePath)
}
