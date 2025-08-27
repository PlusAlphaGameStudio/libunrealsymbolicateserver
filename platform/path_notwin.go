//go:build !windows

package platform

const addr2lineExePath = "~/Library/Android/sdk/ndk/26.2.11394342/toolchains/llvm/prebuilt/darwin-x86_64/bin/llvm-addr2line"
const xcrunExePath = "/usr/bin/xcrun"
const sevenZipExePath = "/usr/local/bin/7z"

func GetAddr2lineExePath() string {
	return replaceHome(addr2lineExePath)
}

func GetXCRunExePath() string {
	return xcrunExePath
}

func GetSevenZipExePath() string {
	return replaceHome(sevenZipExePath)
}

func ExecuteBatchSelfTests(selfTestSingle selfTestSingleFunc) {
	selfTestSingle("samples/testTombstone")
	selfTestSingle("samples/LastUnhandledCrashStack-Android.xml")
	selfTestSingle("samples/LastCrashStack-Android.txt")

	selfTestSingle("samples/LastUnhandledCrashStack-IOS.xml")
}
