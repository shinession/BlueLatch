# English

## ADR 0001: Reconnect a device by switching its audio services off and on again

Status: accepted on 2026-11-24. Verified on one device, a speaker, on 2026-11-24 (see Observed below). Not verified yet on earbuds that also have Hands-Free, nor for the question whether administrator rights are needed (see [runbook.md](../runbook.md)).

### Context

After the first connection, some paired audio devices show "Not connected" in Windows Settings, and **Connect** has to be clicked several times. BlueLatch has to connect such a device and retry by itself. Windows has no documented call that simply connects a classic audio device.

Options:

1. `BluetoothSetServiceState` in the Win32 Bluetooth API (`bthprops.cpl`): switch the audio services of the device off and on again. It is documented, it can be called through syscalls without cgo, and it needs no dependency beyond `golang.org/x/sys/windows`. As far as we know, command-line tools for connecting a paired Bluetooth device on Windows work this way too.
2. The WinRT Bluetooth classes. As far as we know they read the connection status and open RFCOMM or GATT sessions, but have no call that connects a classic audio device. They also need COM/WinRT bindings.
3. Restarting the device node (`pnputil /restart-device`, the Configuration Manager API). It needs administrator rights and resets more than necessary.
4. Automating the Settings page (UI Automation). It is brittle and tied to the Windows language and version.

### Decision

Use option 1. The connection state is read with `BluetoothGetDeviceInfo`. A retry loop (`internal/latch`) repeats the request until the connection holds.

### Observed

A real run of `bluelatch connect -v` on a speaker (A2DP Sink only), pasted by the user on 2026-11-24, from the log:

- The speaker was not connected when the run started. The first two attempts did not connect it within 10 s each. The third attempt did: it was connected at the first read after the request returned, and stayed connected. Connected after 3 attempts in 46.3 s. This is the "several clicks on Connect" of the Settings page, done by the loop.
- One request took about 6.5 s: switching the service off about 3.1 s, the pause of 0.3 s, switching it on about 3.3 s. Both calls returned success every time.
- The A2DP Sink was missing from the list of installed services after it was switched off, and listed again when switching it on returned.

Not known: why the first two attempts did not connect (nothing is known about the state of the speaker at that time), what the calls return when a device is out of reach, and whether the terminal that ran it was elevated.

### Consequences

- No cgo and no dependency beyond `golang.org/x/sys/windows`.
- The call is synchronous and cannot be interrupted. A service that has been switched off must always be switched on again, so `Reconnect` enables each service even when disabling it failed, the engine lets a request finish before it stops, and a journal repairs what a stop in between leaves behind ([ADR 0005](./0005-journal-of-service-switches.md)).
- A request takes about 6.5 s, so an attempt, with the wait for the link, takes about 17 s.
- The Win32 Bluetooth API sees classic devices only (ADR 0002).
- If a later check shows that the method does not work on some device, a later ADR supersedes this one.

---

# 中文

## ADR 0001：通过把音频服务先关闭再打开来重新连接设备

状态：2026-11-24 已接受。2026-11-24 已在一个设备（一台音箱）上验证（见下面的"观察到的情况"）。尚未在同时带有 Hands-Free 的耳塞上验证，也尚未验证是否需要管理员权限（参见 [runbook.md](../runbook.md)）。

### 背景

第一次连接之后，一些已配对的音频设备在 Windows 设置里显示"未连接"，需要多次点击 **Connect**。BlueLatch 要连接这样的设备，并自行重试。Windows 没有文档化的调用可以直接连接经典蓝牙音频设备。

备选方案：

1. Win32 蓝牙 API（`bthprops.cpl`）中的 `BluetoothSetServiceState`：把设备的音频服务先关闭再打开。它有文档，可以通过 syscall 调用而无需 cgo，除 `golang.org/x/sys/windows` 外不需要其他依赖。据我们所知，Windows 上连接已配对蓝牙设备的命令行工具也是这样工作的。
2. WinRT 蓝牙类。据我们所知，它们能读取连接状态，并打开 RFCOMM 或 GATT 会话，但没有连接经典蓝牙音频设备的调用。它们还需要 COM/WinRT 绑定。
3. 重启设备节点（`pnputil /restart-device`、Configuration Manager API）。需要管理员权限，并且重置的内容超出必要范围。
4. 自动化操作设置页面（UI Automation）。脆弱，并且与 Windows 的语言和版本绑定。

### 决定

采用方案 1。连接状态用 `BluetoothGetDeviceInfo` 读取。重试循环（`internal/latch`）重复发出请求，直到连接稳定。

### 观察到的情况

用户于 2026-11-24 贴出的、在一台音箱（只有 A2DP Sink）上真实运行 `bluelatch connect -v` 的日志：

- 运行开始时音箱未连接。前两次尝试各等待 10 秒，都没有连上。第三次尝试连上了：请求返回后的第一次读取就已是连接状态，并保持住了。经过 3 次尝试、用时 46.3 秒连接成功。这就是设置页面里"要点几次 Connect"，由循环自动完成。
- 一个请求约用 6.5 秒：关闭服务约 3.1 秒，停顿 0.3 秒，打开服务约 3.3 秒。两个调用每次都返回成功。
- 服务关闭之后，A2DP Sink 从已安装服务列表中消失；打开服务的调用返回时，它重新出现在列表中。

不清楚的是：前两次尝试为什么没连上（当时音箱处于什么状态不得而知），设备够不着时这些调用会返回什么，以及运行它的终端是否已提权。

### 后果

- 不使用 cgo，除 `golang.org/x/sys/windows` 外没有依赖。
- 这个调用是同步的，不能被中断。被关闭的服务必须总是被重新打开，所以 `Reconnect` 即使在关闭失败时也会启用每个服务，引擎会让一个请求完成之后才停止，并且待恢复记录会修复中途停止所遗留的问题（[ADR 0005](./0005-journal-of-service-switches.md)）。
- 一个请求约 6.5 秒，所以一次尝试连同等待连接，约 17 秒。
- Win32 蓝牙 API 只能看到经典设备（ADR 0002）。
- 如果以后的检查表明该方法在某个设备上行不通，由后来的 ADR 取代本 ADR。
