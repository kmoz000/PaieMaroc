# Third-party notices

## go-zkteco

The read-only ZKTeco TCP attendance adapter in `internal/attendance/device.go`
is adapted from [farizfadian/go-zkteco](https://github.com/farizfadian/go-zkteco),
revision `36209a7dfc0d34e0448309af295581bcd2b77001`.

MIT License

Copyright (c) 2025 Fariz Fadian

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

## wireguard-go

The optional userspace-only tunnel in `internal/pointerservice` uses
[wireguard-go](https://git.zx2c4.com/wireguard-go), copyright WireGuard LLC.
The project is distributed under the MIT License. Its full license text is
available with the source module and at the upstream repository. PaieMaroc
does not install or load a kernel WireGuard driver.

## gVisor netstack

wireguard-go's userspace network stack depends on
[gVisor](https://gvisor.dev/), copyright The gVisor Authors. gVisor is
distributed under the Apache License 2.0. Its license and notices are
available from the upstream project and the corresponding Go module.
