# English

## BlueLatch architecture

This document describes what is implemented in version 0.1.0. What is planned is marked as planned.

### Overview

BlueLatch is one Go executable for Windows, built without cgo. It has a command line only; the window is planned (ADR 0003).

```text
cmd/bluelatch ---> internal/latch (retry engine) ---> bt.Adapter (interface)
      |                                                    |
      |                                                    v
      +---------> internal/bt: types, Find           internal/bt/*_windows.go ---> bthprops.cpl (Win32)
      |
      +---------> internal/logging ---> %LocalAppData%\BlueLatch\bluelatch.log
```

| Package | Responsibility |
|---|---|
| `cmd/bluelatch` | The commands `list`, `connect`, `version`. Picks the device the user named and prints the progress of the engine. |
| `internal/latch` | The retry engine, `Engine.Connect`. It knows nothing about Windows and uses `bt.Adapter` only. |
| `internal/bt` | The types (`Device`, `Address`, `Service`, `ClassOfDevice`), the `Adapter` interface, `Find`, and the Windows implementation of the interface. |
| `internal/logging` | The log file. |

### Reading the devices

`Adapter.Devices` and `Adapter.Device` use these Win32 calls, all from `bthprops.cpl`, called through `golang.org/x/sys/windows`:

| Call | Used for |
|---|---|
| `BluetoothFindFirstRadio`, `BluetoothFindNextRadio`, `BluetoothFindRadioClose` | Open the local radios. No radio is `bt.ErrNoRadio`. Windows is expected to report the same while Bluetooth is turned off (not verified). |
| `BluetoothGetRadioInfo` | The name, address, and manufacturer of the radio, shown by `list -v`. |
| `BluetoothFindFirstDevice`, `BluetoothFindNextDevice`, `BluetoothFindDeviceClose` | List the devices that are authenticated, remembered, or connected. No inquiry is started, so it returns at once. |
| `BluetoothGetDeviceInfo` | Read the current state of one device. |
| `BluetoothEnumerateInstalledServices` | The standard services Windows has installed for a device. Vendor specific 128-bit services are left out. |

Rules built on them:

- A device is *paired* when Windows reports it as authenticated or remembered.
- A device is an *audio device* when its class is Audio/Video, or when A2DP Sink, Hands-Free, or Headset is installed for it.
- The time stamps (`last seen`, `last used`) are local time. Verified on the development PC against the system clock.
- The API lists classic devices only. Verified on the development PC: a paired Bluetooth LE mouse is not listed, not even by `list -all` (ADR 0002).
- The structs passed to the API are the 64-bit layouts of `bluetoothapis.h`. The build checks their sizes and fails on 32-bit Windows (checked: GOARCH 386 fails, amd64 and arm64 build).

### The reconnect request

`Adapter.Reconnect` does this for one device, on the radio that knows it:

1. Read the installed services and keep the ones in this order: A2DP Sink, Hands-Free, Headset. None installed is `bt.ErrNoAudioServices`.
2. For each of them: switch it off (`BluetoothSetServiceState`, disable), wait 300 ms, switch it on (enable). The enable call is made even when the disable call failed, and it is what asks Windows to connect.
3. Return what happened to each service. It is an error only when no service could be enabled. A nil error does not mean the device is connected.

The call is synchronous and cannot be interrupted (ADR 0001).

### The retry engine

`Engine.Connect(ctx, address, report)`:

1. Read the device. A device that is connected already is reported as connected at once, with 0 attempts, and is not touched.
2. Loop:
   1. From the second attempt on, first look whether the device has connected by itself during the pause. If it is connected and holds, finish; a link that is coming up is not disturbed by another request.
   2. Make the reconnect request (phase `reconnecting`).
   3. Wait for the link (phase `waiting`): read the device every `PollInterval` until it has been connected for `StableFor` (success), or until `AttemptTimeout` has passed without a connection. A connection that is up at the deadline is given the time to prove itself.
   4. If `MaxAttempts` is reached, return `ErrGaveUp`. Otherwise pause (phase `backoff`) and start the next attempt.
3. A successful end is reported as phase `connected`.

| Policy field | Default | Meaning |
|---|---|---|
| `AttemptTimeout` | 10 s | How long one attempt waits for the link after Windows has answered. |
| `PollInterval` | 500 ms | How often the device state is read while waiting. |
| `StableFor` | 2 s | How long the device must stay connected to count as connected. Zero accepts the first connected reading. |
| `BackoffStart`, `BackoffMax` | 2 s, 5 s | The first pause, and the limit. Each pause is half as long again as the one before. |
| `MaxAttempts` | 0 | 0 means no limit. |

Ways out of the loop:

- Success: the device holds its connection.
- The context is cancelled (the user stops it). A request that Windows is working on is not abandoned: it switches services off and on again and must run to its end, so cancelling takes effect when the request returns.
- A fatal error: `bt.ErrNoRadio`, `bt.ErrDeviceNotFound`, or `bt.ErrNoAudioServices`. Retrying cannot fix them.
- `ErrGaveUp` after `MaxAttempts` attempts.

Any other error from a request is not fatal: it is shown, and the engine waits for the link and tries again. A failed read of the device state counts as "not connected" for that read.

### Logging

`internal/logging` writes `%LocalAppData%\BlueLatch\bluelatch.log` (text, English). Above 1 MiB the file is moved to `bluelatch.log.1` when the program starts (one previous log is kept). Every request is logged with the outcome of each service, including the Windows error codes. `-v` adds a line for every read of the device state, and also writes to stderr.

### Planned, not built

- The window (ADR 0003): a WebView2 window that shows the same states as `latch.Phase` and starts `Engine.Connect` for the device whose **Connect** button is clicked, with a **Cancel** button.

### Not verified yet

- Whether the reconnect request connects a device that is in reach, and what it returns and how long it blocks when the device is out of reach (ADR 0001).
- Whether the request needs administrator rights. Listing was verified in a process that is not elevated.
- Whether `Connected` matches the state Windows Settings shows ("Connected audio").
- Whether Windows reports no radio (`bt.ErrNoRadio`) while Bluetooth is turned off. It was not tried, because turning Bluetooth off disconnects the devices in use.

---

# 中文

## BlueLatch 架构

本文档描述 0.1.0 版中已实现的内容。计划中的内容会标明是计划。

### 概览

BlueLatch 是一个 Windows 下的 Go 可执行文件，不使用 cgo 构建。目前只有命令行；窗口是计划中的内容（ADR 0003）。

```text
cmd/bluelatch ---> internal/latch (retry engine) ---> bt.Adapter (interface)
      |                                                    |
      |                                                    v
      +---------> internal/bt: types, Find           internal/bt/*_windows.go ---> bthprops.cpl (Win32)
      |
      +---------> internal/logging ---> %LocalAppData%\BlueLatch\bluelatch.log
```

| 包 | 职责 |
|---|---|
| `cmd/bluelatch` | 命令 `list`、`connect`、`version`。找出用户指定的设备，并打印引擎的进度。 |
| `internal/latch` | 重试引擎 `Engine.Connect`。它不了解 Windows，只使用 `bt.Adapter`。 |
| `internal/bt` | 类型（`Device`、`Address`、`Service`、`ClassOfDevice`）、`Adapter` 接口、`Find`，以及该接口的 Windows 实现。 |
| `internal/logging` | 日志文件。 |

### 读取设备

`Adapter.Devices` 和 `Adapter.Device` 使用下面这些 Win32 调用，全部来自 `bthprops.cpl`，通过 `golang.org/x/sys/windows` 调用：

| 调用 | 用途 |
|---|---|
| `BluetoothFindFirstRadio`、`BluetoothFindNextRadio`、`BluetoothFindRadioClose` | 打开本机的蓝牙适配器。没有适配器是 `bt.ErrNoRadio`。蓝牙关闭时预计 Windows 也会报告同样的结果（尚未验证）。 |
| `BluetoothGetRadioInfo` | 适配器的名称、地址和厂商，由 `list -v` 显示。 |
| `BluetoothFindFirstDevice`、`BluetoothFindNextDevice`、`BluetoothFindDeviceClose` | 列出已认证、已记住或已连接的设备。不会发起查询（inquiry），所以立即返回。 |
| `BluetoothGetDeviceInfo` | 读取一个设备的当前状态。 |
| `BluetoothEnumerateInstalledServices` | Windows 为设备安装的标准服务。厂商自定义的 128 位服务不包含在内。 |

基于它们的规则：

- Windows 报告设备为已认证或已记住时，设备就是*已配对*的。
- 设备的类别是 Audio/Video，或者为它安装了 A2DP Sink、Hands-Free 或 Headset，它就是*音频设备*。
- 时间戳（`last seen`、`last used`）是本地时间。已在开发用电脑上与系统时钟对照验证。
- 这个 API 只列出经典设备。已在开发用电脑上验证：已配对的蓝牙 LE 鼠标不会被列出，连 `list -all` 也不会（ADR 0002）。
- 传给 API 的结构体是 `bluetoothapis.h` 的 64 位布局。构建时会检查它们的大小，在 32 位 Windows 上会失败（已检查：GOARCH 386 失败，amd64 和 arm64 可以构建）。

### 重新连接请求

`Adapter.Reconnect` 对一个设备，在认识它的适配器上执行以下步骤：

1. 读取已安装的服务，按这个顺序保留其中的服务：A2DP Sink、Hands-Free、Headset。一个都没有安装是 `bt.ErrNoAudioServices`。
2. 对其中每一个：关闭（`BluetoothSetServiceState`，disable），等待 300 毫秒，再打开（enable）。即使 disable 调用失败，也会执行 enable 调用，请求 Windows 去连接的正是它。
3. 返回每个服务的处理结果。只有一个服务都没能启用时才算错误。错误为 nil 并不表示设备已连接。

这个调用是同步的，不能被中断（ADR 0001）。

### 重试引擎

`Engine.Connect(ctx, address, report)`：

1. 读取设备。已经连接的设备会立即被报告为已连接，尝试次数为 0，并且不会被触碰。
2. 循环：
   1. 从第二次尝试起，先看设备是否在停顿期间自己连上了。如果它已连接并保持住，就结束；正在建立的连接不会被再发一次请求所打扰。
   2. 发出重新连接请求（阶段 `reconnecting`）。
   3. 等待连接（阶段 `waiting`）：每隔 `PollInterval` 读取一次设备，直到它连续保持连接 `StableFor`（成功），或者 `AttemptTimeout` 过去仍没有连接。在截止时刻已经连上的连接，会被给予证明自己的时间。
   4. 如果达到 `MaxAttempts`，返回 `ErrGaveUp`。否则停顿（阶段 `backoff`），开始下一次尝试。
3. 成功结束时报告阶段 `connected`。

| Policy 字段 | 默认值 | 含义 |
|---|---|---|
| `AttemptTimeout` | 10 秒 | Windows 应答之后，一次尝试等待连接建立的时间。 |
| `PollInterval` | 500 毫秒 | 等待期间读取设备状态的频率。 |
| `StableFor` | 2 秒 | 设备必须保持连接多久才算已连接。0 表示接受第一次读到的已连接状态。 |
| `BackoffStart`、`BackoffMax` | 2 秒、5 秒 | 第一次停顿的时长和上限。每次停顿比前一次长一半。 |
| `MaxAttempts` | 0 | 0 表示不限制。 |

退出循环的方式：

- 成功：设备保持住了连接。
- 上下文被取消（用户停止）。Windows 正在处理的请求不会被放弃：它会把服务先关闭再打开，必须执行完，所以取消在请求返回时才生效。
- 致命错误：`bt.ErrNoRadio`、`bt.ErrDeviceNotFound` 或 `bt.ErrNoAudioServices`。重试无法修复它们。
- 达到 `MaxAttempts` 次尝试后返回 `ErrGaveUp`。

请求返回的其他错误都不是致命的：会显示出来，引擎仍会等待连接并再次尝试。读取设备状态失败时，该次读取按"未连接"处理。

### 日志

`internal/logging` 写入 `%LocalAppData%\BlueLatch\bluelatch.log`（文本，英文）。程序启动时，如果文件超过 1 MiB，会被移到 `bluelatch.log.1`（保留一份旧日志）。每次请求都会记录每个服务的结果，包括 Windows 错误码。`-v` 会为每次读取设备状态增加一行，并且同时写到 stderr。

### 计划中，尚未构建

- 窗口（ADR 0003）：一个 WebView2 窗口，显示与 `latch.Phase` 相同的状态，为被点击 **Connect** 按钮的设备启动 `Engine.Connect`，并带一个 **Cancel** 按钮。

### 尚未验证

- 重新连接请求能否连接够得着的设备，以及设备够不着时它返回什么、阻塞多久（ADR 0001）。
- 该请求是否需要管理员权限。列出设备已在未提权的进程中验证过。
- `Connected` 是否与 Windows 设置显示的状态（"Connected audio"）一致。
- 蓝牙关闭时 Windows 是否报告没有适配器（`bt.ErrNoRadio`）。没有试过，因为关闭蓝牙会断开正在使用的设备。
