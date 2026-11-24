# English

## BlueLatch runbook

### Development setup

- Go 1.27 or newer (developed with 1.27.4). `CGO_ENABLED` is 0 and there is no C compiler on the development PC; none is needed.
- Windows with a Bluetooth radio, for the live checks.

```powershell
go build ./...
go vet ./...
gofmt -l .                      # prints nothing when all is formatted
go test ./... -count=1
```

`go test -race` does not work, because the project builds without cgo.

### Build

```powershell
.\scripts\build.ps1 -Version 0.1.0   # writes release\bluelatch.exe, 3 MB, static
```

### Live checks

**1. Reading (always safe).** It changes nothing.

```powershell
$env:BLUELATCH_LIVE = '1'; go test ./internal/bt -run TestLiveDevices -v -count=1
.\release\bluelatch.exe list -v
```

Result on the development PC (2026-11-24): the paired audio devices are listed with their services, in an elevated process and in a normal one (integrity level Medium). A paired Bluetooth LE mouse is not listed (ADR 0002). `connect` to a device that was connected already returned at once without touching it.

**2. The reconnect request (changes the state of a device; do it only with the user).**

1. Choose a paired audio device that nobody is using. Put it in its case or switch it off, so that Windows shows it as "Not connected".
2. Start the program from a process that is **not elevated**. A shell that is elevated hides a possible need for administrator rights. From an elevated shell, a script opened with `explorer.exe` runs with the normal token; its first line of output can show the integrity level (`whoami /groups`).

   ```text
   bluelatch connect -v "<device name>"
   ```

3. While the program is retrying, take the device out of its case or switch it on.
4. Write down: whether it connected, after which attempt and how long, the line `Windows answered (...)` of every attempt with its Windows error codes, and what Windows Settings shows ("Connected audio"). The log file has the same, with every state read when `-v` is used.
5. Afterwards check that no service stayed switched off: `bluelatch list -v` must still list the services of the device (A2DP Sink and the others). Windows Settings must still offer **Connect**.

Result: **not done yet.** The answers to the questions in architecture.md ("Not verified yet") are to be written here.

### The log file

`%LocalAppData%\BlueLatch\bluelatch.log`. Above 1 MiB it is moved to `bluelatch.log.1` when the program starts. `connect` prints the path on its first line. Send the log when a device will not connect: it holds every request with the Windows error codes.

### Troubleshooting

| Symptom | Cause and action |
|---|---|
| `no Bluetooth radio found (is Bluetooth turned on?)` | The PC has no adapter, or Bluetooth is turned off (expected, not verified). Turn Bluetooth on in Windows Settings. |
| `the device is not paired` | Windows holds no pairing for the device, for example after it was removed in Settings. Pair it again in Windows Settings. |
| `the device has no installed audio service` | Windows has installed none of A2DP Sink, Hands-Free, or Headset for the device. Remove the device in Windows Settings and pair it again. |
| `No paired Bluetooth audio devices found` | There is none, or the device is a Bluetooth LE device, which is not listed (ADR 0002). `list -all` shows every classic device Windows reports. |
| `... is paired but is not an audio device` | `connect` handles audio devices only (ADR 0002). |
| `Windows answered (... enable failed: ... (error N))` | Windows refused the request. The meaning of the codes seen in practice is to be recorded in this table. |

### Releasing

Not set up yet.

---

# 中文

## BlueLatch 运行手册

### 开发环境

- Go 1.27 或更高版本（用 1.27.4 开发）。`CGO_ENABLED` 为 0，开发用电脑上没有 C 编译器，也不需要。
- 带蓝牙适配器的 Windows，用于真机检查。

```powershell
go build ./...
go vet ./...
gofmt -l .                      # prints nothing when all is formatted
go test ./... -count=1
```

`go test -race` 无法使用，因为项目不使用 cgo 构建。

### 构建

```powershell
.\scripts\build.ps1 -Version 0.1.0   # writes release\bluelatch.exe, 3 MB, static
```

### 真机检查

**1. 读取（始终安全）。**不会改变任何东西。

```powershell
$env:BLUELATCH_LIVE = '1'; go test ./internal/bt -run TestLiveDevices -v -count=1
.\release\bluelatch.exe list -v
```

在开发用电脑上的结果（2026-11-24）：在已提权的进程和普通进程（完整性级别 Medium）中，都能列出已配对的音频设备及其服务。已配对的蓝牙 LE 鼠标不会被列出（ADR 0002）。对已经连接的设备执行 `connect`，会立即返回，并且不会触碰设备。

**2. 重新连接请求（会改变设备的状态；只能与用户一起做）。**

1. 选一个已配对且没人在使用的音频设备。把它放回充电盒或关机，让 Windows 显示它为"未连接"。
2. 从**未提权**的进程启动程序。已提权的 shell 会掩盖可能需要管理员权限的情况。在已提权的 shell 中，用 `explorer.exe` 打开的脚本会以普通令牌运行；它输出的第一行可以显示完整性级别（`whoami /groups`）。

   ```text
   bluelatch connect -v "<device name>"
   ```

3. 程序重试期间，把设备从充电盒里取出来或开机。
4. 记录：是否连上、在第几次尝试之后、用了多久、每次尝试的 `Windows answered (...)` 一行及其 Windows 错误码，以及 Windows 设置显示的状态（"Connected audio"）。日志文件里有同样的内容，使用 `-v` 时还有每次读取状态的记录。
5. 事后检查没有服务一直处于关闭状态：`bluelatch list -v` 必须仍然列出该设备的服务（A2DP Sink 等）。Windows 设置里也必须仍然提供 **Connect**。

结果：**尚未做。**architecture.md 中（"尚未验证"）那些问题的答案，将写在这里。

### 日志文件

`%LocalAppData%\BlueLatch\bluelatch.log`。程序启动时，如果超过 1 MiB，会被移到 `bluelatch.log.1`。`connect` 在第一行打印它的路径。设备连不上时请把日志发来：它记录了每次请求及其 Windows 错误码。

### 故障排查

| 现象 | 原因与处理 |
|---|---|
| `no Bluetooth radio found (is Bluetooth turned on?)` | 电脑没有适配器，或蓝牙已关闭（预期如此，尚未验证）。在 Windows 设置中打开蓝牙。 |
| `the device is not paired` | Windows 没有该设备的配对，例如它在设置中被删除了。在 Windows 设置中重新配对。 |
| `the device has no installed audio service` | Windows 没有为该设备安装 A2DP Sink、Hands-Free 或 Headset 中的任何一个。在 Windows 设置中删除该设备并重新配对。 |
| `No paired Bluetooth audio devices found` | 确实没有，或者设备是蓝牙 LE 设备，它们不会被列出（ADR 0002）。`list -all` 显示 Windows 报告的每个经典设备。 |
| `... is paired but is not an audio device` | `connect` 只处理音频设备（ADR 0002）。 |
| `Windows answered (... enable failed: ... (error N))` | Windows 拒绝了请求。实际遇到的错误码的含义，将记录在这张表里。 |

### 发布

尚未建立。
