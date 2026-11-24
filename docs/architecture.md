# English

## BlueLatch architecture

This document describes what is implemented in version 0.1.3. What is planned is marked as planned.

### Overview

BlueLatch is one Go program for Windows, built without cgo (ADR 0008): `bluelatch.exe`. It is the command line (`list`, `connect`, `disconnect`, `version`) and the window (`bluelatch` without a command, or `bluelatch window`).

```text
bluelatch.exe   (cmd/bluelatch, one console program)
      |                                      |
  command line                          window
  list, connect, disconnect             bluelatch, bluelatch window
      |                                      |
      |                              internal/ui   WebView2 window, web/index.html
      |                                      |
      |                              internal/app  device list, one job
      |                                      |
      +--------> internal/latch  (retry engine)  <--------+
                        |
                 bt.Adapter  (interface, internal/bt)
                        |
      internal/bt/*_windows.go ---> bthprops.cpl (Win32)

internal/logging, internal/appdir:  %LocalAppData%\BlueLatch\  (log, journal, WebView2 data)
```

| Package | Responsibility |
|---|---|
| `cmd/bluelatch` | The program. The commands `list`, `connect`, `disconnect`, `version`, `help`: they pick the device the user named and print the progress of the engine. And the start of the window (`window.go`, `window_windows.go`, `console_windows.go`): how the process is attached to a console, the flags, single instance, start-up repair, then `ui.Run`. |
| `internal/ui` | The WebView2 window and its page (`web/index.html`, embedded). |
| `internal/app` | What the window shows and does: the list of paired audio devices, kept fresh, and at most one job (a connection or a disconnection). Also the demo adapter. |
| `internal/latch` | The retry engine, `Engine.Connect`. It knows nothing about Windows and uses `bt.Adapter` only. |
| `internal/bt` | The types (`Device`, `Address`, `Service`, `ClassOfDevice`), the `Adapter` interface, `Find`, the journal, and the Windows implementation of the interface. |
| `internal/logging` | The log file. |
| `internal/appdir` | The folder `%LocalAppData%\BlueLatch`. |
| `assets/logo` | The source of the icon (`bluelatch.svg`) and its PNG frames. |
| `tools/mkrsrc` | Writes the `.syso` files that give the program its icon and version information. Standard library only. |

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
2. For each of them: write a journal record, switch the service off (`BluetoothSetServiceState`, disable), wait 300 ms, switch it on (enable), and delete the record when that succeeded. The enable call is made even when the disable call failed, and it is what asks Windows to connect.
3. Return what happened to each service. It is an error only when no service could be enabled. A nil error does not mean the device is connected.

The call is synchronous and cannot be interrupted (ADR 0001). Observed once, on a speaker: switching off about 3.1 s, switching on about 3.3 s, so a request takes about 6.5 s. The service is missing from the list of installed services while it is off.

### The disconnect request

`Adapter.Disconnect` sends `IOCTL_BTH_DISCONNECT_DEVICE` (`0x0041000C`), with the 64-bit address of the device as its input, to the handle of the radio that knows the device (`DeviceIoControl`). It drops the baseband connection; the pairing and the services stay as they are (ADR 0007). Windows answers a device that is not connected with `ERROR_DEVICE_NOT_CONNECTED` (1167), which becomes `bt.ErrNotConnected`. No other code is taken for it.

`Engine.Disconnect` reads the device first. A device that is not connected is reported as such (`bt.ErrNotConnected`) and nothing is sent. Otherwise it sends the request and reads the device every `PollInterval` until Windows reports it as disconnected (success), or until `DisconnectTimeout` (5 s) has passed (`ErrStillConnected`). The request returns at once and switches nothing off, so it needs no record in the journal, and stopping the wait leaves nothing half done.

### The journal

The journal is the folder `%LocalAppData%\BlueLatch\pending`: one small file per service switch in progress (ADR 0005). `Recover` runs at the start of `bluelatch connect` and of the window, before anything is connected. It switches on the services that files still describe, deletes the files, and leaves alone the file of a process that is still running and is younger than two minutes. `list` does not recover.

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

### The application layer

`internal/app` keeps what the window shows. `App.Start` reads the devices once and then every two seconds; `App.Snapshot` returns the state, which is rendered as JSON:

- `devices`: the paired audio devices with their name, address, kind (`headphones`, `headset`, `speaker`, `audio`, from the minor class), and whether they are connected.
- `problem`: `no-radio`, or `error` with the text, when the devices cannot be read. A change of the problem is logged, not every read.
- `job`: the connection being retried or dropped, or the last one: its kind (`connect` or `disconnect`), device, phase (`starting`, `reconnecting`, `waiting`, `backoff`, `cancelling`, `connected` for a connection; `disconnecting`, `disconnected` for a disconnection; `cancelled` and `failed` for both), attempt, start time, the time of the next attempt, the detail of what Windows did, the error, and an error code (`no-radio`, `not-paired`, `no-services`, `gave-up`, `still-connected`).
- `closing`: the program is shutting down.

`Connect` and `Disconnect` start a job and return at once. Only one job runs at a time (`ErrBusy`), of either kind. A disconnection that finds the device not connected counts as done. `Cancel` marks the job as `cancelling` and cancels its context. `Shutdown(timeout)` cancels the job and the reading, and waits for the request that Windows is working on; it reports false after the timeout. `NewDemo` is an adapter with made-up devices for trying the window.

### How the program starts

`bluelatch.exe` is a console program on purpose (ADR 0008). A shell waits for a console program and sees its exit code, which the command line needs. The window then has to get rid of the console that Windows gives a console program when it is started by a double click.

`bluelatch` without a command and `bluelatch window` call `openWindow`. It asks how the process is attached to a console (`GetConsoleProcessList`):

| State | Meaning | What happens |
|---|---|---|
| own (1 process) | Started by a double click, from a shortcut, or from the Start menu: Windows made a console window for this process alone. | `FreeConsole` is the first thing done, which closes that window, and the window of BlueLatch opens in this process. Problems are shown in a message box. |
| shared (2 or more) | Started from a terminal. | The flags are checked: a mistake is written to stderr with exit code 2, and `-h` prints the usage and exits with 0. Then the program starts itself again as `bluelatch window ...` (`DETACHED_PROCESS`, so without a console, with `BLUELATCH_DETACHED=1` in its environment) and returns 0, so the prompt comes back at once. |
| none (0) | There is no console: the process that was started for a terminal, or a scheduled task. | The window opens in this process. Problems are shown in a message box. |

`BLUELATCH_DETACHED` makes a process count as "none" whatever its console is, so the process that was started for a terminal can never start another one. The other commands never touch the console.

Measured on the development PC (Windows 11 Enterprise LTSC 10.0.26100, Windows Terminal 1.25 as the default terminal): a program started by Explorer is alone on its console, and one started by `cmd.exe` shares it. After a double click the window of Windows Terminal is visible for about 30 ms before `FreeConsole` closes it (30 ms and 31 ms in the two runs with this program, 25 to 31 ms in three runs of a probe). Without `FreeConsole` it stays for as long as the program runs. Not measured: a PC without Windows Terminal (the classic console host).

### The window

Once the window runs, the process has no console. It logs to the log file (and sends the log of the WebView2 binding there), takes a named mutex so that only one window runs (a second start brings the open one to the front), runs the repair of the journal, starts the application, and calls `ui.Run`.

The flags of `bluelatch window`:

| Flag | Meaning |
|---|---|
| `-lang auto|en|zh` | The language of the window. `auto` follows Windows (a language that starts with `zh` is Chinese, any other is English). |
| `-v` | Log every read of the device state. |
| `-debug` | Switch on the developer tools and the context menu. |
| `-demo` | Use the made-up devices of `app.NewDemo`; no real device is touched. The title is "BlueLatch (demo)". |
| `-demo-connect NAME` | With `-demo`, start connecting that device 1.5 s after the start. |
| `-demo-disconnect NAME` | With `-demo`, start disconnecting that device 1.5 s after the start. |

`ui.Run` uses `github.com/jchv/go-webview2` (ADR 0003):

- The page is `web/index.html`, embedded in the executable. It loads nothing from outside (content security policy), and sets the names of devices as text.
- Four functions are bound: `blState()`, `blConnect(address)`, `blDisconnect(address)`, `blCancel()`. They return at once. The page calls `blState` every 500 ms and draws the cards from the state; a second timer updates the elapsed time and the countdown. A finished job disappears from its card after 8 s (30 s for a failure).
- A card has one button: Cancel while that device is being connected, Disconnect when it is connected, Connect otherwise. While a job runs, the buttons of the other cards are disabled. The header of the page shows the icon, and the title bar and the taskbar button get the icon of the program in the size that fits the DPI of the window (`setWindowIcons`).
- The page follows the dark or light theme of Windows. The title bar is switched to dark when the apps of the user use the dark theme. The process is per-monitor DPI aware and the window is scaled to the system DPI. WebView2 keeps its data in `%LocalAppData%\BlueLatch\WebView2`.
- The page blocks reload (F5, Ctrl+R), print, save, find, zoom, and dropped files, because the page was loaded from a string and a reload would empty it.
- Closing the window: the first `WM_CLOSE` calls `App.Shutdown` (at most 30 s) and is ignored; when it has returned, the window is closed for real. A request that Windows is working on therefore always runs to its end.
- When the window cannot be created, usually because the WebView2 runtime is missing, a message box names the download page.

### The icon and the version information

`tools/mkrsrc` writes a `.syso` file for each architecture (`cmd/bluelatch/rsrc_windows_amd64.syso` and `rsrc_windows_arm64.syso`). It is a COFF object with a `.rsrc` section that holds an RT_GROUP_ICON with the ID 1, one RT_ICON for every frame in `assets/logo/png` up to 256 px (a bitmap for the small ones, the PNG file itself for 256 px), and an RT_VERSION. The Go linker links the file by itself, so every build has them. `scripts\resources.ps1 -Version <version>` regenerates the two files, and `scripts\logo-frames.ps1` renders the frames from the SVG (ADR 0006). There is no manifest.

### Logging

`internal/logging` writes `%LocalAppData%\BlueLatch\bluelatch.log` (text, English). Above 1 MiB the file is moved to `bluelatch.log.1` when the program starts (one previous log is kept). Every request is logged with the outcome of each service, including the Windows error codes. `-v` adds a line for every read of the device state, and the command line also writes to stderr.

### Not verified yet

- Earbuds that have Hands-Free as well as A2DP: the one real run was on a speaker with A2DP only.
- Whether the reconnect request needs administrator rights. Listing was verified in a process that is not elevated. It is not known whether the terminal of the one real run, or the window of the runs by the user, was elevated.
- What the calls return, and how long they block, for a device that is out of reach.
- Whether `Connected` matches the state Windows Settings shows ("Connected audio").
- The Cancel button of the window against a real device. It was exercised in demo mode only. The user clicked Connect and Disconnect in the window on a speaker; the log of that run is in ADR 0007 and in check 4 of the runbook.
- Disconnect on the earbuds (A2DP and Hands-Free), and on a device that connects again by itself. On the speaker the request was accepted at once, Windows reported the speaker as disconnected at the first read, 0.5 s later, and the speaker had not connected again by itself when the Connect that followed came, about 5 s later (ADR 0007).
- Whether Windows reports no radio (`bt.ErrNoRadio`) while Bluetooth is turned off. It was not tried, because turning Bluetooth off disconnects the devices in use.
- A PC with more than one Bluetooth radio.
- How long the console window shows after a double click on a PC without Windows Terminal, and on another PC in general (ADR 0008).

---

# 中文

## BlueLatch 架构

本文档描述 0.1.3 版中已实现的内容。计划中的内容会标明是计划。

### 概览

BlueLatch 是一个 Windows 下的 Go 程序，不使用 cgo 构建（ADR 0008）：`bluelatch.exe`。它既是命令行（`list`、`connect`、`disconnect`、`version`），也是窗口（不带命令的 `bluelatch`，或者 `bluelatch window`）。

```text
bluelatch.exe   (cmd/bluelatch, one console program)
      |                                      |
  command line                          window
  list, connect, disconnect             bluelatch, bluelatch window
      |                                      |
      |                              internal/ui   WebView2 window, web/index.html
      |                                      |
      |                              internal/app  device list, one job
      |                                      |
      +--------> internal/latch  (retry engine)  <--------+
                        |
                 bt.Adapter  (interface, internal/bt)
                        |
      internal/bt/*_windows.go ---> bthprops.cpl (Win32)

internal/logging, internal/appdir:  %LocalAppData%\BlueLatch\  (log, journal, WebView2 data)
```

| 包 | 职责 |
|---|---|
| `cmd/bluelatch` | 程序本身。命令 `list`、`connect`、`disconnect`、`version`、`help`：它们找出用户指定的设备，并打印引擎的进度。还有窗口的启动（`window.go`、`window_windows.go`、`console_windows.go`）：进程如何附着在控制台上、参数、单实例、启动时的修复，然后调用 `ui.Run`。 |
| `internal/ui` | WebView2 窗口及其页面（`web/index.html`，已嵌入）。 |
| `internal/app` | 窗口显示和执行的内容：已配对音频设备的列表（保持刷新），以及至多一个任务（连接或断开）。还有演示用的 adapter。 |
| `internal/latch` | 重试引擎 `Engine.Connect`。它不了解 Windows，只使用 `bt.Adapter`。 |
| `internal/bt` | 类型（`Device`、`Address`、`Service`、`ClassOfDevice`）、`Adapter` 接口、`Find`、待恢复记录，以及该接口的 Windows 实现。 |
| `internal/logging` | 日志文件。 |
| `internal/appdir` | 文件夹 `%LocalAppData%\BlueLatch`。 |
| `assets/logo` | 图标的源文件（`bluelatch.svg`）及其 PNG 帧。 |
| `tools/mkrsrc` | 写出让程序拥有图标和版本信息的 `.syso` 文件。只用标准库。 |

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
2. 对其中每一个：写入一条待恢复记录，关闭服务（`BluetoothSetServiceState`，disable），等待 300 毫秒，再打开（enable），打开成功后删除记录。即使 disable 调用失败，也会执行 enable 调用，请求 Windows 去连接的正是它。
3. 返回每个服务的处理结果。只有一个服务都没能启用时才算错误。错误为 nil 并不表示设备已连接。

这个调用是同步的，不能被中断（ADR 0001）。在一台音箱上观察到一次：关闭约 3.1 秒，打开约 3.3 秒，所以一个请求约 6.5 秒。服务关闭期间，它不在已安装服务列表中。

### 断开请求

`Adapter.Disconnect` 用 `DeviceIoControl` 向认识该设备的适配器的句柄发送 `IOCTL_BTH_DISCONNECT_DEVICE`（`0x0041000C`），输入是设备的 64 位地址。它断开基带连接；配对和服务保持不变（ADR 0007）。Windows 对未连接的设备返回 `ERROR_DEVICE_NOT_CONNECTED`（1167），它会变成 `bt.ErrNotConnected`。不会把其他错误码当作它。

`Engine.Disconnect` 先读取设备。设备未连接时，如实报告（`bt.ErrNotConnected`），什么也不发送。否则发出请求，并每隔 `PollInterval` 读取一次设备，直到 Windows 报告它已断开（成功），或 `DisconnectTimeout`（5 秒）过去（`ErrStillConnected`）。请求会立即返回，也不关闭任何东西，所以不需要待恢复记录，停止等待也不会留下做到一半的事。

### 待恢复记录

待恢复记录（journal）是文件夹 `%LocalAppData%\BlueLatch\pending`：每个正在进行的服务开关对应一个小文件（ADR 0005）。`Recover` 在 `bluelatch connect` 和窗口启动时、连接任何设备之前运行。它把文件仍然描述的服务重新打开，删除文件，并且不处理这样的文件：写入它的进程仍在运行，并且文件不到两分钟。`list` 不执行恢复。

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

### 应用层

`internal/app` 保存窗口显示的内容。`App.Start` 先读取一次设备，之后每两秒读取一次；`App.Snapshot` 返回状态，状态以 JSON 渲染：

- `devices`：已配对的音频设备，含名称、地址、类型（`headphones`、`headset`、`speaker`、`audio`，由次要类别得出），以及是否已连接。
- `problem`：无法读取设备时为 `no-radio`，或带文本的 `error`。问题发生变化时才记录日志，而不是每次读取都记录。
- `job`：正在重试或正在断开的连接，或最近一次的：种类（`connect` 或 `disconnect`）、设备、阶段（连接用 `starting`、`reconnecting`、`waiting`、`backoff`、`cancelling`、`connected`；断开用 `disconnecting`、`disconnected`；两者都可能是 `cancelled` 和 `failed`）、尝试次数、开始时间、下一次尝试的时间、Windows 所做操作的详情、错误，以及错误码（`no-radio`、`not-paired`、`no-services`、`gave-up`、`still-connected`）。
- `closing`：程序正在关闭。

`Connect` 和 `Disconnect` 启动一个任务并立即返回。同一时间只运行一个任务（`ErrBusy`），不论哪种。断开时发现设备并未连接，也算完成。`Cancel` 把任务标记为 `cancelling` 并取消它的上下文。`Shutdown(timeout)` 取消任务和读取，并等待 Windows 正在处理的请求；超时后返回 false。`NewDemo` 是带虚构设备的 adapter，用于试用窗口。

### 程序如何启动

`bluelatch.exe` 有意做成控制台程序（ADR 0008）。shell 会等待控制台程序并拿到它的退出码，命令行需要这一点。窗口则必须摆脱 Windows 在双击启动控制台程序时给它的那个控制台。

不带命令的 `bluelatch` 和 `bluelatch window` 会调用 `openWindow`。它先问进程是如何附着在控制台上的（`GetConsoleProcessList`）：

| 状态 | 含义 | 发生什么 |
|---|---|---|
| own（1 个进程） | 通过双击、快捷方式或开始菜单启动：Windows 只为这个进程创建了一个控制台窗口。 | 第一件事就是 `FreeConsole`，这会关闭那个窗口，然后 BlueLatch 的窗口在本进程中打开。问题显示在消息框里。 |
| shared（2 个或更多） | 从终端启动。 | 先检查参数：有错就写到 stderr 并返回退出码 2，`-h` 打印用法并返回 0。然后程序以 `bluelatch window ...` 再启动自己一次（`DETACHED_PROCESS`，即没有控制台，环境中带有 `BLUELATCH_DETACHED=1`），并返回 0，所以提示符立即回来。 |
| none（0 个） | 没有控制台：为终端启动的那个进程，或者计划任务。 | 窗口在本进程中打开。问题显示在消息框里。 |

`BLUELATCH_DETACHED` 让进程不管控制台是什么情况都按 "none" 处理，所以为终端启动的那个进程绝不会再启动另一个。其他命令从不触碰控制台。

在开发用电脑上的测量（Windows 11 Enterprise LTSC 10.0.26100，Windows Terminal 1.25 为默认终端）：由 Explorer 启动的程序独占它的控制台，由 `cmd.exe` 启动的则与之共享。双击之后，Windows Terminal 的窗口在 `FreeConsole` 把它关闭之前可见约 30 毫秒（用本程序跑了两次，分别是 30 毫秒和 31 毫秒；用探测程序跑了三次，是 25 到 31 毫秒）。不调用 `FreeConsole` 的话，它会一直留到程序结束。没有测量过：没有 Windows Terminal 的电脑（经典控制台宿主）。

### 窗口

窗口运行起来之后，进程没有控制台。它把日志写入日志文件（WebView2 绑定的日志也送到那里），获取一个命名互斥量以保证只运行一个窗口（第二次启动会把已打开的窗口带到前台），运行待恢复记录的修复，启动应用，然后调用 `ui.Run`。

`bluelatch window` 的参数：

| 参数 | 含义 |
|---|---|
| `-lang auto|en|zh` | 窗口的语言。`auto` 跟随 Windows（以 `zh` 开头的语言为中文，其他都是英文）。 |
| `-v` | 记录每次读取设备状态。 |
| `-debug` | 打开开发者工具和右键菜单。 |
| `-demo` | 使用 `app.NewDemo` 的虚构设备，不触碰任何真实设备。标题为 "BlueLatch (demo)"。 |
| `-demo-connect NAME` | 与 `-demo` 一起使用，启动 1.5 秒后开始连接该设备。 |
| `-demo-disconnect NAME` | 与 `-demo` 一起使用，启动 1.5 秒后开始断开该设备。 |

`ui.Run` 使用 `github.com/jchv/go-webview2`（ADR 0003）：

- 页面是 `web/index.html`，嵌入在可执行文件中。它不从外部加载任何内容（内容安全策略），并且以文本方式设置设备名称。
- 绑定了四个函数：`blState()`、`blConnect(address)`、`blDisconnect(address)`、`blCancel()`。它们都立即返回。页面每 500 毫秒调用一次 `blState`，根据状态绘制卡片；另一个定时器更新已用时间和倒计时。已结束的任务在 8 秒后从卡片上消失（失败的是 30 秒）。
- 每张卡片有一个按钮：该设备正在连接时是 Cancel，已连接时是 Disconnect，其他情况是 Connect。有任务运行时，其他卡片的按钮被禁用。页面头部显示图标，标题栏和任务栏按钮则按窗口所在的 DPI 取程序图标中合适的尺寸（`setWindowIcons`）。
- 页面跟随 Windows 的深色或浅色主题。当用户的应用使用深色主题时，标题栏切换为深色。进程是 per-monitor DPI 感知的，窗口按系统 DPI 缩放。WebView2 的数据保存在 `%LocalAppData%\BlueLatch\WebView2`。
- 页面会阻止刷新（F5、Ctrl+R）、打印、保存、查找、缩放和拖入文件，因为页面是从字符串加载的，刷新会让它变空。
- 关闭窗口：第一个 `WM_CLOSE` 调用 `App.Shutdown`（最多 30 秒）并被忽略；它返回之后，窗口才真正关闭。因此 Windows 正在处理的请求总会执行到结束。
- 窗口无法创建时（通常是缺少 WebView2 运行时），会弹出一个消息框，其中给出下载页面。

### 图标和版本信息

`tools/mkrsrc` 为每种架构写出一个 `.syso` 文件（`cmd/bluelatch/rsrc_windows_amd64.syso` 和 `rsrc_windows_arm64.syso`）。它是带有 `.rsrc` 节的 COFF 目标文件，其中包含 ID 为 1 的 RT_GROUP_ICON，`assets/logo/png` 中每个不超过 256 像素的帧各对应一个 RT_ICON（小尺寸用位图，256 像素用 PNG 文件本身），以及一个 RT_VERSION。Go 链接器会自动链接这个文件，所以每次构建都带有它们。`scripts\resources.ps1 -Version <version>` 重新生成这两个文件，`scripts\logo-frames.ps1` 从 SVG 渲染出这些帧（ADR 0006）。没有 manifest。

### 日志

`internal/logging` 写入 `%LocalAppData%\BlueLatch\bluelatch.log`（文本，英文）。程序启动时，如果文件超过 1 MiB，会被移到 `bluelatch.log.1`（保留一份旧日志）。每次请求都会记录每个服务的结果，包括 Windows 错误码。`-v` 会为每次读取设备状态增加一行，命令行还会同时写到 stderr。

### 尚未验证

- 既有 A2DP 又有 Hands-Free 的耳塞：唯一一次真实运行是在只有 A2DP 的音箱上。
- 重新连接请求是否需要管理员权限。列出设备已在未提权的进程中验证过。那次真实运行所用的终端、以及用户运行窗口时的进程是否已提权，不得而知。
- 设备够不着时，这些调用返回什么、阻塞多久。
- `Connected` 是否与 Windows 设置显示的状态（"Connected audio"）一致。
- 窗口的 Cancel 按钮对真实设备的效果。它只在演示模式下试过。用户在窗口里对一台音箱点击过 Connect 和 Disconnect；那次运行的日志记录在 ADR 0007 和运行手册的检查 4 中。
- 在耳塞（A2DP 加 Hands-Free）上执行 Disconnect，以及在会自己再连上的设备上执行。在音箱上，请求被立即接受，Windows 在第一次读取时（0.5 秒后）就报告音箱已断开，而随后的 Connect 到来时（约 5 秒后）音箱并没有自己再连上（ADR 0007）。
- 蓝牙关闭时 Windows 是否报告没有适配器（`bt.ErrNoRadio`）。没有试过，因为关闭蓝牙会断开正在使用的设备。
- 有多个蓝牙适配器的电脑。
- 在没有 Windows Terminal 的电脑上，以及一般意义上的其他电脑上，双击之后控制台窗口会显示多久（ADR 0008）。
