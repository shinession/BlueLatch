# English

## ADR 0001: Reconnect a device by switching its audio services off and on again

Status: accepted on 2026-11-24. Not verified yet on a device that is out of reach (see the live check in [runbook.md](../runbook.md)).

### Context

After the first connection, some paired audio devices show "Not connected" in Windows Settings, and **Connect** has to be clicked several times. BlueLatch has to connect such a device and retry by itself. Windows has no documented call that simply connects a classic audio device.

Options:

1. `BluetoothSetServiceState` in the Win32 Bluetooth API (`bthprops.cpl`): switch the audio services of the device off and on again. It is documented, it can be called through syscalls without cgo, and it needs no dependency beyond `golang.org/x/sys/windows`. As far as we know, command-line tools for connecting a paired Bluetooth device on Windows work this way too.
2. The WinRT Bluetooth classes. As far as we know they read the connection status and open RFCOMM or GATT sessions, but have no call that connects a classic audio device. They also need COM/WinRT bindings.
3. Restarting the device node (`pnputil /restart-device`, the Configuration Manager API). It needs administrator rights and resets more than necessary.
4. Automating the Settings page (UI Automation). It is brittle and tied to the Windows language and version.

### Decision

Use option 1. The connection state is read with `BluetoothGetDeviceInfo`. A retry loop (`internal/latch`) repeats the request until the connection holds.

### Consequences

- No cgo and no dependency beyond `golang.org/x/sys/windows`.
- The call is synchronous and cannot be interrupted. A service that has been switched off must always be switched on again, so `Reconnect` enables each service even when disabling it failed, and the engine lets a request finish before it stops.
- The Win32 Bluetooth API sees classic devices only (ADR 0002).
- Not known until the live check: whether the request connects a device that is in reach, how long it blocks and what it returns when the device is not in reach, and whether it needs administrator rights. If the live check shows that it does not work, a later ADR supersedes this one.

---

# 中文

## ADR 0001：通过把音频服务先关闭再打开来重新连接设备

状态：2026-11-24 已接受。尚未在够不着的设备上验证（参见 [runbook.md](../runbook.md) 中的真机检查）。

### 背景

第一次连接之后，一些已配对的音频设备在 Windows 设置里显示"未连接"，需要多次点击 **Connect**。BlueLatch 要连接这样的设备，并自行重试。Windows 没有文档化的调用可以直接连接经典蓝牙音频设备。

备选方案：

1. Win32 蓝牙 API（`bthprops.cpl`）中的 `BluetoothSetServiceState`：把设备的音频服务先关闭再打开。它有文档，可以通过 syscall 调用而无需 cgo，除 `golang.org/x/sys/windows` 外不需要其他依赖。据我们所知，Windows 上连接已配对蓝牙设备的命令行工具也是这样工作的。
2. WinRT 蓝牙类。据我们所知，它们能读取连接状态，并打开 RFCOMM 或 GATT 会话，但没有连接经典蓝牙音频设备的调用。它们还需要 COM/WinRT 绑定。
3. 重启设备节点（`pnputil /restart-device`、Configuration Manager API）。需要管理员权限，并且重置的内容超出必要范围。
4. 自动化操作设置页面（UI Automation）。脆弱，并且与 Windows 的语言和版本绑定。

### 决定

采用方案 1。连接状态用 `BluetoothGetDeviceInfo` 读取。重试循环（`internal/latch`）重复发出请求，直到连接稳定。

### 后果

- 不使用 cgo，除 `golang.org/x/sys/windows` 外没有依赖。
- 这个调用是同步的，不能被中断。被关闭的服务必须总是被重新打开，所以 `Reconnect` 即使在关闭失败时也会启用每个服务，引擎也会让一个请求完成之后才停止。
- Win32 蓝牙 API 只能看到经典设备（ADR 0002）。
- 在真机检查之前不清楚的是：请求能否连接够得着的设备，设备够不着时它会阻塞多久、返回什么，以及它是否需要管理员权限。如果真机检查表明它行不通，由后来的 ADR 取代本 ADR。
