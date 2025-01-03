//go:build windows

package platform

const Addr2lineExePath = "~\\AppData\\Local\\Android\\Sdk\\ndk\\26.2.11394342\\toolchains\\llvm\\prebuilt\\windows-x86_64\\bin\\llvm-addr2line.exe"
const Cmd = "cat '%s' | Select-String -Pattern '^ libUnreal' | Foreach-Object { $_.ToString().split('+')[1] } | %s -C -f -e C:\\Users\\gb\\Downloads\\libUnreal.so"