# English

## BlueLatch runbook

### Development setup

- Go 1.27 or newer (developed with 1.27.4). `CGO_ENABLED` is 0 and there is no C compiler on the development PC; none is needed.
- Windows with a Bluetooth radio for the live checks, and the WebView2 runtime to run the window (it is part of Windows 11).

```powershell
go build ./...
go vet ./...
gofmt -l .                      # prints nothing when all is formatted
go test ./... -count=1
```

`go test -race` does not work, because the project builds without cgo.

### Build

```powershell
.\scripts\build.ps1 -Version 0.1.4   # writes release\bluelatch.exe
```

`bluelatch.exe` is the whole program, the command line and the window (4.2 MB). It is static and needs nothing next to it. It is a console program on purpose (ADR 0008), so do not build it with `-H=windowsgui`. The icon and the version information are part of it: the files `cmd\bluelatch\rsrc_windows_*.syso` are committed and linked by `go build` (see "Changing the icon or the version").

### Running the window

Start `release\bluelatch.exe` by a double click or from a shortcut, or type `bluelatch` (or `bluelatch window`) in a terminal. From a terminal the prompt comes back at once, because the window opens in another process (ADR 0008). After a double click a console window shows for about 30 ms before the program closes it. The window needs the WebView2 runtime; without it a message box names the download page.

The flags of `bluelatch window`:

| Flag | Meaning |
|---|---|
| `-lang auto|en|zh` | The language of the window; `auto` follows Windows. |
| `-v` | Log every read of the device state. |
| `-debug` | Developer tools and context menu. |
| `-demo` | Made-up devices; no real device is touched. |
| `-demo-connect NAME` | With `-demo`, start connecting that device 1.5 s after the start. |
| `-demo-disconnect NAME` | With `-demo`, start disconnecting that device 1.5 s after the start. |

**Demo mode** shows the whole window without a real device: `bluelatch window -demo -demo-connect "Demo Earbuds"`. The earbuds connect on the third request, after about 34 s. Demo Headphones never connect, which is how to try **Cancel**. The demo speaker is connected: `-demo -demo-disconnect "Demo Speaker"` disconnects it, which takes about a second, and the **Disconnect** button does the same. A device that was disconnected needs its requests again (the earbuds three). Closing the window during a request waits for that request, as it does with a real device.

The demo mode has made-up sound outputs too, and no real output is touched: a monitor, and one for each connected demo device. **Switch** moves the check mark. Disconnecting the demo speaker removes its output, and the monitor takes over; the output of the earbuds appears once they are connected.

A second start of the window does not open a second window; it brings the open one to the front.

### Checking the page without Windows

`internal/ui/web/index.html` talks to Go only through `window.blState`, `window.blConnect`, `window.blDisconnect`, `window.blCancel`, and `window.blSwitch`. To look at a state, put a `<script>` that defines these five (and `window.blLang`) before the script of the page, and open the file in headless Chrome, for example with `chrome --headless=new --screenshot=out.png --window-size=600,700 --virtual-time-budget=1500 page.html`. Below a width of about 500, headless Chrome cuts the right edge off the picture. The state is a JSON object as described in architecture.md; times are Unix milliseconds, so write them as `Date.now() - 46300` in the mock. This was how the dark and light themes, English and Chinese, and the states idle, waiting, backoff, connected, failed, no radio, and no devices were checked for 0.1.1. For 0.1.2, the Chinese texts and the failed disconnection were checked in the same way.

To see the window itself without clicking by hand, `PrintWindow` of `user32.dll` with the flag 2 draws it into a bitmap, and the UI Automation `InvokePattern` clicks its buttons by their accessible names (`Switch <output name>`, `Connect <device name>`).

### Changing the icon or the version

The icon is `assets/logo/bluelatch.svg` (ADR 0006). When it changes, or when the version changes:

```powershell
.\scripts\logo-frames.ps1               # only when the SVG changed: renders assets\logo\png\bluelatch-N.png (needs Chrome)
.\scripts\resources.ps1 -Version 0.1.4  # writes cmd\bluelatch\rsrc_windows_*.syso, which are committed
.\scripts\build.ps1 -Version 0.1.4
```

Give the two scripts the same version as `build.ps1`: `resources.ps1` sets the version information of the files, `build.ps1` the version that `bluelatch version` and the footer of the window show. To check the result, `[System.Diagnostics.FileVersionInfo]::GetVersionInfo("release\bluelatch.exe")` shows the version information, and Windows can extract the icon of a program at a given size with `PrivateExtractIcons` of `user32.dll`.

### Live checks

**1. Reading (always safe).** It changes nothing.

```powershell
$env:BLUELATCH_LIVE = '1'; go test ./internal/bt -run TestLiveDevices -v -count=1
go test ./internal/audio -run TestLiveOutputs -v -count=1
.\release\bluelatch.exe list -v
```

Result on the development PC (2026-11-24): the paired audio devices are listed with their services, in an elevated process and in a normal one (integrity level Medium). A paired Bluetooth LE mouse is not listed (ADR 0002). `connect` to a device that was connected already returned at once without touching it. The window, started as a normal user, listed the same devices. The outputs test is new in 0.1.4: on 2026-11-28, from a process that was not elevated, it listed the two active outputs, a Bluetooth speaker and a monitor, with the speaker as the default of all three roles, and could create the object for `IPolicyConfig`; the whole test took 0.01 s.

**2. The reconnect request (changes the state of a device; do it only with the user).**

1. Choose a paired audio device that nobody is using. Put it in its case or switch it off, so that Windows shows it as "Not connected".
2. Start the program from a process that is **not elevated**. A shell that is elevated hides a possible need for administrator rights. From an elevated shell, a script opened with `explorer.exe` runs with the normal token; its first line of output can show the integrity level (`whoami /groups`).

   ```text
   bluelatch connect -v "<device name>"
   ```

3. While the program is retrying, take the device out of its case or switch it on.
4. Write down: whether it connected, after which attempt and how long, the line `Windows answered (...)` of every attempt with its Windows error codes, and what Windows Settings shows ("Connected audio"). The log file has the same, and with `-v` the services Windows lists around each switch.
5. Afterwards check that no service stayed switched off: `bluelatch list -v` must still list the services of the device (A2DP Sink and the others), and the folder `%LocalAppData%\BlueLatch\pending` must be empty or absent. Windows Settings must still offer **Connect**.

Result (2026-11-24): done once by the user, on a speaker (A2DP Sink only), and pasted into the conversation. It was not connected at the start. Attempts 1 and 2 did not connect it within 10 s each; attempt 3 did, at the first read after the request returned. Connected after 3 attempts in 46.3 s. Each request took about 6.5 s (off about 3.1 s, on about 3.3 s, both returning success), and the A2DP Sink was missing from the list while it was off. The details are in ADR 0001. Not done yet: earbuds that also have Hands-Free, and a run from a process that is known not to be elevated.

**3. The window against a real device (changes the state of a device; do it only with the user).**

1. Start `bluelatch.exe` as a normal user (a double click). Choose a device that nobody is using and that Windows shows as "Not connected".
2. Click **Connect**. The card shows "Connecting…", the attempt, the elapsed time and what Windows did. Take the device out of its case. The card must turn to "Connected" and say after how many attempts.
3. Click **Connect** again on another disconnected device and **Cancel** while a request is in progress. The card must say "Stopping… finishing the current request." and then end within about 7 s.
4. Close the window while a request is in progress. It must stay open until the request has returned (about 7 s at most), then close; the log must show "the reconnect request was answered" before "the window was closed".
5. Afterwards: `bluelatch list -v` lists the services of the device, and the folder `%LocalAppData%\BlueLatch\pending` is empty or absent.

Result: partly done. The user clicked **Connect** in the window on a speaker twice on 2026-11-24, with 0.1.1 at 22:27 and with 0.1.2 at 23:21. The log of the first run shows the request, the connection on attempt 1 after 8.4 s, and the window closed afterwards; the second run is in check 4. Not done with a real device: **Cancel**, closing the window during a request, and the earbuds. Done in demo mode (below).

**4. Disconnect (changes the state of a device; do it only with the user).**

1. Choose a connected audio device that nobody is using at that moment, such as a speaker that is not playing.
2. Start the program from a process that is **not elevated**, as in check 2, and run `bluelatch disconnect -v "<device name>"`, or click **Disconnect** on its card in the window.
3. Write down: whether Windows Settings shows the device as "Not connected" at once; how long it takes until `bluelatch list` says so (the command waits up to 5 s, and fails with "still reports being connected" when the device stays connected); the line "the disconnect request was accepted" in the log; and whether the device connects again by itself within a minute.
4. Connect it again with **Connect** (or `bluelatch connect`), and check that it connects and that `bluelatch list -v` still lists its services. Nothing has to be repaired, because a disconnection switches no service off.

Result (2026-11-24): done once by the user, on a speaker, in the window of 0.1.2 at 23:21; the log is the record. Disconnect: the request was accepted at once, and the speaker was reported as disconnected at the first read, 0.5 s later. Connect, about five seconds later: it found the speaker not connected, so the speaker had not connected again by itself in those five seconds; it switched the A2DP Sink service off and on and connected the speaker on the first attempt after 8.4 s. The folder `pending` was empty afterwards. The log does not say whether the window ran elevated, and what Windows Settings showed is not recorded. Before that, the request had been sent as a normal user (integrity level Medium) for earbuds that were not connected: Windows answered ERROR_DEVICE_NOT_CONNECTED (1167), not an access error. Not done yet: the earbuds, `bluelatch disconnect` on a connected device, and a wait of a minute to see whether a device connects again by itself.

**5. The checks of 0.1.1 that changed no real device (2026-11-24).**

- Window in demo mode, started as a normal user through Explorer: the earbuds connected after 3 attempts in 34.0 s, with the progress shown on the card.
- Closing while idle: the window was gone after 0.03 s. Closing during a request: it was gone after 1.08 s, and the log shows the request answered at 22:18:29.405 and the window closed at 22:18:29.423.
- Window with the real adapter, started as a normal user, read-only: it listed both real devices with the right state, closed in 0.05 s, and left no folder `pending` behind.
- The window was sharp and the right size on a scaled display (it measured 864 x 1219 pixels), and had the dark title bar.

**6. The checks of 0.1.2 that changed no real device (2026-11-24).**

- Window in demo mode, started as a normal user through Explorer, with `-demo-disconnect "Demo Speaker"`: before, the speaker card has a Disconnect button; during, it shows "Disconnecting…" with its own button and the buttons of the other cards disabled; afterwards, "Not connected", a Connect button, and "Disconnected. The device stays paired." The Chinese texts and the failure "The device still reports being connected" were checked with a mock back end.
- Window with the real adapter, started as a normal user, read-only: the connected speaker shows a Disconnect button and the earbuds a Connect button. Nothing was clicked, and the folder `pending` stayed absent.
- `bluelatch disconnect` on the earbuds that were not connected: "is not connected", exit code 0, and no request sent. With a name that matches nothing: exit code 1. Without a name: exit code 2.
- The icon: both programs link with the resource files; `FileVersionInfo` reads 0.1.2 and the names; Windows extracts the icon at 16, 24, 32, 48, and 256 px; the title bar of the window and the header of its page show it.

**7. The checks of 0.1.3 that changed no real device (2026-11-24).**

- Double click: Explorer started `release\bluelatch.exe` without arguments (the real adapter, nothing clicked), and the visible top-level windows were polled every 2 ms. A window of Windows Terminal was there for 30 ms (from 372 ms to 402 ms after the launch), then only the window "BlueLatch" stayed, for the 4.5 s that were watched. One process; closing the window ended it, and the log shows "starting the window" and "the window was closed".
- A shortcut with the arguments `window -demo`, started in the same way: the same, with 31 ms, and the window "BlueLatch (demo)".
- Typed in a terminal (the PowerShell of the tool): `bluelatch window -demo` returned after 44 ms with exit code 0, and the window "BlueLatch (demo)" was the only new window, with no terminal window. A second `bluelatch window -demo` returned after 53 ms and left one process: the open window was brought to the front. Through `cmd /c`, `bluelatch window -demo` returned after 64 ms.
- The command line is still a console program: through `cmd /v:on /c`, `bluelatch disconnect` without a name left the errorlevel 2. In PowerShell `$LASTEXITCODE` was 2 as well, also for `connect` without a name, for an unknown command, and for `window -lang fr`, which wrote the mistake and the usage to stderr. `window -h` printed the usage with exit code 0, `version` and `help` worked, and `list` listed the devices.
- The window of the one program, drawn into a picture with `PrintWindow` (started with `-demo-disconnect "Demo Speaker"`): the title bar with the icon, the header with the logo, the three demo devices, "Disconnected. The device stays paired." on the speaker, and "v0.1.3" in the footer.
- `FileVersionInfo` of `release\bluelatch.exe` reads 0.1.3, product "BlueLatch", description "BlueLatch", original file name `bluelatch.exe`. The subsystem in the PE header is 3 (console).
- The unit tests cover the three console states with fakes, and a real `StartDetached`: the test binary starts itself again, and the new process reports 0 console processes and the variable `BLUELATCH_DETACHED`.

**8. Switching the sound output (changes a setting of Windows; do it only with the user).**

1. Choose a moment when nothing important is playing. Note the default output of each role with the outputs test of check 1.
2. From a process that is **not elevated**, run `bluelatch switch -v` (with two outputs, to the other one).
3. Check with the outputs test that the console and multimedia roles moved, and that the calls moved if they were on the old output.
4. Switch back with `bluelatch switch -v "<name of the first output>"`, and check that all three roles are as at the start.
5. Write down the lines "changed the default output" of the log, and what the output shows.

Result (2026-11-28), with the permission of the user: from a process that was not elevated, `switch` moved all three roles from a Bluetooth speaker to a monitor ("Switched the sound output from ... to ...", exit code 0), the outputs test showed the monitor for all three roles, and `switch` with the name of the speaker moved all three back. The two switches were less than a second apart. Afterwards `switch` with the name of the current output printed "is already the sound output." and changed nothing, and a name that matches nothing failed with exit code 1 and listed the outputs. Not done: the **Switch** button of the window against the real outputs, calls on a separate output, and a headset with Hands-Free.

**9. The checks of 0.1.4 that changed no real device and no real output (2026-11-28).**

- Window in demo mode, in Chinese: the two made-up outputs with the speaker checked; **Switch** on the monitor moved the check mark, and back; after **Disconnect** of the demo speaker its output was gone and the monitor was the current one; after the demo earbuds connected (3 attempts, 34.0 s) their output appeared. The log shows "changed the default output" for each switch.
- The window with the real adapters, read-only, nothing clicked: it listed the two real outputs with the speaker checked, with a long name of an output on one line.
- The page with a mock back end in headless Chrome: dark and light theme, English and Chinese, a switch in progress, a failed switch, outputs that cannot be read, no outputs, and an output name that is HTML and has `{name}` in it, which was shown as text.

### The log file

`%LocalAppData%\BlueLatch\bluelatch.log`, written by the program. Above 1 MiB it is moved to `bluelatch.log.1` when the program starts. `connect` prints the path on its first line. Send the log when a device will not connect: it holds every request with the Windows error codes.

### Troubleshooting

| Symptom | Cause and action |
|---|---|
| `no Bluetooth radio found (is Bluetooth turned on?)`, or the red banner in the window | The PC has no adapter, or Bluetooth is turned off (expected, not verified). Turn Bluetooth on in Windows Settings. |
| `the device is not paired` | Windows holds no pairing for the device, for example after it was removed in Settings. Pair it again in Windows Settings. |
| `the device has no installed audio service` | Windows has installed none of A2DP Sink, Hands-Free, or Headset for the device. If BlueLatch was stopped in the middle of a request, start `bluelatch connect` or the window once: it switches the service on again (ADR 0005). Otherwise remove the device in Windows Settings and pair it again. |
| `Switched on the ... service of ... again; an earlier run had left it switched off.` | Not a problem: an earlier run was stopped between switching a service off and on, and this start repaired it. |
| `No paired Bluetooth audio devices found`, or the empty window | There is none, or the device is a Bluetooth LE device, which is not listed (ADR 0002). `list -all` shows every classic device Windows reports. |
| `... is paired but is not an audio device` | `connect` and `disconnect` handle audio devices only (ADR 0002). |
| `... is not connected.` (from `disconnect`) | Nothing to do: the device was not connected. |
| `... still reports being connected ... It may have connected again by itself.` | The disconnect request was accepted, but the device went on reporting being connected, or connected again at once. Some devices reconnect by themselves when they are dropped (ADR 0007). Switch the device off, or connect it to something else, and try again. |
| `Windows answered (... enable failed: ... (error N))` | Windows refused the request. The meaning of the codes seen in practice is to be recorded in this table. |
| A message box about the WebView2 runtime | The window needs the Microsoft Edge WebView2 Runtime, from the page the message names. |
| The window does not open a second time | Another window is open, and was brought to the front. |
| Closing the window takes several seconds | A request that Windows is working on is finishing first (ADR 0005). |
| A console or terminal window flashes for a moment when the window opens | Windows makes a console for a console program that is started by a double click or from a shortcut, and the program closes it at once: about 30 ms with Windows Terminal (ADR 0008). Started from a terminal, there is no flash. |
| `bluelatch` opens the window instead of printing the usage | Since 0.1.3, `bluelatch` without a command opens the window. The usage is `bluelatch help`. |
| `bluelatch: the window could not be started: ...` | Started from a terminal, the program could not start itself again for the window. The text says why. A double click opens the window in the same process. |
| No window appears after `bluelatch` in a terminal | The window opens in another process, so a problem there does not show in the terminal. It is shown in a message box and written to the log file. |
| `no sound output matches ...` | The name is not among the outputs that are active now; the message lists them. A Bluetooth speaker that is not connected is not one. |
| `bluelatch switch: there is no other sound output to switch to` | Only one output is active. Connect or plug in another one. |
| `Windows did not change the default output` (or, in the window, "Windows did not change the output.") | Windows accepted the change but another output is still the default. Send the log. The sound flyout of Windows still works. |
| `Windows does not offer a way to change the default output: ...` | The `IPolicyConfig` interface is not there, for example after a Windows update (ADR 0009). Use the sound flyout of Windows. Send the log with the HRESULT. |
| `making ... the default output (...): IPolicyConfig.SetDefaultEndpoint: ... (HRESULT 0x...)` | Windows refused the change. The meaning of the codes seen in practice is to be recorded in this table. |
| Sound does not follow a switch | An app was given an output of its own in the volume mixer, or it keeps playing on the old one until it is restarted. That is up to the app (not tried). |

### Releasing

Not set up yet.

---

# 中文

## BlueLatch 运行手册

### 开发环境

- Go 1.27 或更高版本（用 1.27.4 开发）。`CGO_ENABLED` 为 0，开发用电脑上没有 C 编译器，也不需要。
- 真机检查需要带蓝牙适配器的 Windows，运行窗口需要 WebView2 运行时（Windows 11 自带）。

```powershell
go build ./...
go vet ./...
gofmt -l .                      # prints nothing when all is formatted
go test ./... -count=1
```

`go test -race` 无法使用，因为项目不使用 cgo 构建。

### 构建

```powershell
.\scripts\build.ps1 -Version 0.1.4   # writes release\bluelatch.exe
```

`bluelatch.exe` 是整个程序，既是命令行也是窗口（4.2 MB）。它是静态的，旁边不需要任何其他文件。它有意做成控制台程序（ADR 0008），所以不要用 `-H=windowsgui` 构建它。图标和版本信息就在它里面：文件 `cmd\bluelatch\rsrc_windows_*.syso` 已提交，由 `go build` 链接（见"更换图标或版本"）。

### 运行窗口

双击 `release\bluelatch.exe`，或通过快捷方式启动，或者在终端里输入 `bluelatch`（或 `bluelatch window`）。在终端里启动时提示符会立即回来，因为窗口在另一个进程中打开（ADR 0008）。双击之后会有一个控制台窗口显示约 30 毫秒，随后被程序关闭。窗口需要 WebView2 运行时；缺少时会弹出消息框，其中给出下载页面。

`bluelatch window` 的参数：

| 参数 | 含义 |
|---|---|
| `-lang auto|en|zh` | 窗口的语言；`auto` 跟随 Windows。 |
| `-v` | 记录每次读取设备状态。 |
| `-debug` | 开发者工具和右键菜单。 |
| `-demo` | 使用虚构的设备；不触碰任何真实设备。 |
| `-demo-connect NAME` | 与 `-demo` 一起使用，启动 1.5 秒后开始连接该设备。 |
| `-demo-disconnect NAME` | 与 `-demo` 一起使用，启动 1.5 秒后开始断开该设备。 |

**演示模式**不需要真实设备就能看到完整的窗口：`bluelatch window -demo -demo-connect "Demo Earbuds"`。Demo Earbuds 在第三个请求时连上，约 34 秒。Demo Headphones 永远连不上，用它来试 **Cancel**。Demo Speaker 是已连接的：`-demo -demo-disconnect "Demo Speaker"` 会断开它，约需一秒，**Disconnect** 按钮做的是同一件事。被断开的设备需要重新发出请求（Demo Earbuds 要三次）。在请求进行中关闭窗口，会像真实设备一样等待该请求。

演示模式也有虚构的声音输出，不触碰任何真实输出：一台显示器，以及每个已连接的演示设备各一个。**Switch** 会移动对勾。断开 Demo Speaker 会移除它的输出，由显示器接替；Demo Earbuds 连上后，它的输出会出现。

第二次启动窗口不会打开第二个窗口，而是把已打开的窗口带到前台。

### 不借助 Windows 检查页面

`internal/ui/web/index.html` 只通过 `window.blState`、`window.blConnect`、`window.blDisconnect`、`window.blCancel` 和 `window.blSwitch` 与 Go 通信。要查看某个状态，在页面脚本之前放一个定义这五个函数（以及 `window.blLang`）的 `<script>`，然后用无头 Chrome 打开该文件，例如 `chrome --headless=new --screenshot=out.png --window-size=600,700 --virtual-time-budget=1500 page.html`。宽度小于约 500 时，无头 Chrome 会把图片右侧裁掉。状态是 architecture.md 中描述的 JSON 对象；时间是 Unix 毫秒，所以在模拟数据中写成 `Date.now() - 46300`。0.1.1 的深色和浅色主题、英文和中文，以及空闲、等待、退避、已连接、失败、没有适配器和没有设备这些状态，就是这样检查的。0.1.2 的中文文案和断开失败的状态也是这样检查的。

要在不手动点击的情况下查看窗口本身，可以用 `user32.dll` 的 `PrintWindow`（标志 2）把它画到位图里，用 UI Automation 的 `InvokePattern` 按可访问名称点击按钮（`Switch <output name>`、`Connect <device name>`）。

### 更换图标或版本

图标是 `assets/logo/bluelatch.svg`（ADR 0006）。图标或版本变化时：

```powershell
.\scripts\logo-frames.ps1               # only when the SVG changed: renders assets\logo\png\bluelatch-N.png (needs Chrome)
.\scripts\resources.ps1 -Version 0.1.4  # writes cmd\bluelatch\rsrc_windows_*.syso, which are committed
.\scripts\build.ps1 -Version 0.1.4
```

给两个脚本使用与 `build.ps1` 相同的版本：`resources.ps1` 设置文件的版本信息，`build.ps1` 设置 `bluelatch version` 和窗口页脚显示的版本。要检查结果，`[System.Diagnostics.FileVersionInfo]::GetVersionInfo("release\bluelatch.exe")` 会显示版本信息，Windows 可以用 `user32.dll` 的 `PrivateExtractIcons` 取出程序在指定尺寸下的图标。

### 真机检查

**1. 读取（始终安全）。**不会改变任何东西。

```powershell
$env:BLUELATCH_LIVE = '1'; go test ./internal/bt -run TestLiveDevices -v -count=1
go test ./internal/audio -run TestLiveOutputs -v -count=1
.\release\bluelatch.exe list -v
```

在开发用电脑上的结果（2026-11-24）：在已提权的进程和普通进程（完整性级别 Medium）中，都能列出已配对的音频设备及其服务。已配对的蓝牙 LE 鼠标不会被列出（ADR 0002）。对已经连接的设备执行 `connect`，会立即返回，并且不会触碰设备。以普通用户启动的窗口列出了同样的设备。输出测试是 0.1.4 新增的：2026-11-28 从未提权的进程运行，它列出了两个可用输出（蓝牙音箱和显示器），音箱是三个角色的默认输出，并能创建 `IPolicyConfig` 对象；整个测试用时 0.01 秒。

**2. 重新连接请求（会改变设备的状态；只能与用户一起做）。**

1. 选一个已配对且没人在使用的音频设备。把它放回充电盒或关机，让 Windows 显示它为"未连接"。
2. 从**未提权**的进程启动程序。已提权的 shell 会掩盖可能需要管理员权限的情况。在已提权的 shell 中，用 `explorer.exe` 打开的脚本会以普通令牌运行；它输出的第一行可以显示完整性级别（`whoami /groups`）。

   ```text
   bluelatch connect -v "<device name>"
   ```

3. 程序重试期间，把设备从充电盒里取出来或开机。
4. 记录：是否连上、在第几次尝试之后、用了多久、每次尝试的 `Windows answered (...)` 一行及其 Windows 错误码，以及 Windows 设置显示的状态（"Connected audio"）。日志文件里有同样的内容，使用 `-v` 时还有 Windows 在每次开关前后列出的服务。
5. 事后检查没有服务一直处于关闭状态：`bluelatch list -v` 必须仍然列出该设备的服务（A2DP Sink 等），文件夹 `%LocalAppData%\BlueLatch\pending` 必须为空或不存在。Windows 设置里也必须仍然提供 **Connect**。

结果（2026-11-24）：由用户做过一次，在一台音箱（只有 A2DP Sink）上，并贴到了对话中。开始时它未连接。第 1、2 次尝试各等待 10 秒，都没有连上；第 3 次连上了，是在请求返回后的第一次读取。经过 3 次尝试、用时 46.3 秒连接成功。每个请求约 6.5 秒（关闭约 3.1 秒，打开约 3.3 秒，两者都返回成功），服务关闭期间 A2DP Sink 不在列表中。详情见 ADR 0001。尚未做的：同时带有 Hands-Free 的耳塞，以及在确认未提权的进程中运行。

**3. 窗口对真实设备的检查（会改变设备的状态；只能与用户一起做）。**

1. 以普通用户启动 `bluelatch.exe`（双击）。选一个没人在用、Windows 显示为"未连接"的设备。
2. 点击 **Connect**。卡片显示"Connecting…"、尝试次数、已用时间和 Windows 所做的操作。把设备从充电盒里取出来。卡片必须变成"Connected"，并说明经过了几次尝试。
3. 在另一个未连接的设备上点击 **Connect**，并在请求进行中点击 **Cancel**。卡片必须显示"Stopping… finishing the current request."，然后在约 7 秒内结束。
4. 在请求进行中关闭窗口。它必须保持打开直到请求返回（最多约 7 秒），然后关闭；日志中 "the reconnect request was answered" 必须出现在 "the window was closed" 之前。
5. 事后：`bluelatch list -v` 列出该设备的服务，文件夹 `%LocalAppData%\BlueLatch\pending` 为空或不存在。

结果：部分完成。用户在 2026-11-24 在窗口里对一台音箱点击过两次 **Connect**，分别是 22:27 的 0.1.1 和 23:21 的 0.1.2。第一次运行的日志显示了请求、第 1 次尝试就在 8.4 秒后连上，以及之后窗口被关闭；第二次运行见检查 4。对真实设备尚未做的：**Cancel**、请求进行中关闭窗口，以及耳塞。演示模式下已做过（见下）。

**4. Disconnect（会改变设备的状态；只能与用户一起做）。**

1. 选一个已连接、此刻没人在用的音频设备，例如没有在播放的音箱。
2. 像检查 2 那样，从**未提权**的进程启动程序，运行 `bluelatch disconnect -v "<device name>"`，或在窗口中点击它卡片上的 **Disconnect**。
3. 记录：Windows 设置是否立刻把设备显示为"未连接"；`bluelatch list` 过多久才这样显示（命令最多等待 5 秒，设备一直保持连接就以 "still reports being connected" 失败）；日志中的 "the disconnect request was accepted" 一行；以及设备是否会在一分钟内自己再连上。
4. 用 **Connect**（或 `bluelatch connect`）再次连接它，检查它能连上，并且 `bluelatch list -v` 仍然列出它的服务。不需要修复任何东西，因为断开不会关闭任何服务。

结果（2026-11-24）：由用户做过一次，在一台音箱上，在 23:21 的 0.1.2 窗口里；日志就是记录。Disconnect：请求被立即接受，0.5 秒后第一次读取时音箱就被报告为已断开。约五秒后的 Connect：它发现音箱未连接，所以音箱在这五秒内没有自己再连上；它把 A2DP Sink 服务先关闭再打开，第一次尝试就在 8.4 秒后连上了音箱。事后文件夹 `pending` 为空。日志没有说明窗口是否以提权方式运行，Windows 设置当时显示什么也没有记录。在那之前，曾以普通用户（完整性级别 Medium）对一副未连接的耳塞发送过该请求：Windows 返回 ERROR_DEVICE_NOT_CONNECTED（1167），而不是访问错误。尚未做的：耳塞，在已连接的设备上执行 `bluelatch disconnect`，以及等待一分钟看设备是否会自己再连上。

**5. 0.1.1 中没有改变任何真实设备的检查（2026-11-24）。**

- 窗口在演示模式下，通过 Explorer 以普通用户启动：Demo Earbuds 经过 3 次尝试、用时 34.0 秒连上，进度显示在卡片上。
- 空闲时关闭：0.03 秒后窗口消失。请求进行中关闭：1.08 秒后消失，日志显示请求在 22:18:29.405 返回，窗口在 22:18:29.423 关闭。
- 窗口使用真实 adapter，以普通用户启动，只读：列出了两个真实设备及其正确状态，0.05 秒内关闭，没有留下文件夹 `pending`。
- 在缩放过的显示器上（窗口尺寸为 864 x 1219 像素），窗口清晰、大小合适，并且标题栏是深色。

**6. 0.1.2 中没有改变任何真实设备的检查（2026-11-24）。**

- 窗口在演示模式下，通过 Explorer 以普通用户启动，使用 `-demo-disconnect "Demo Speaker"`：断开前，音箱卡片有 Disconnect 按钮；断开中，显示"Disconnecting…"，它自己的按钮和其他卡片的按钮都被禁用；断开后，显示"Not connected"、Connect 按钮和"Disconnected. The device stays paired."。中文文案和失败状态 "The device still reports being connected" 用模拟后端检查过。
- 窗口使用真实 adapter，以普通用户启动，只读：已连接的音箱显示 Disconnect 按钮，耳塞显示 Connect 按钮。没有点击任何东西，文件夹 `pending` 始终不存在。
- 对未连接的耳塞运行 `bluelatch disconnect`："is not connected"，退出码 0，没有发送请求。名称什么也匹配不上时：退出码 1。没有给名称时：退出码 2。
- 图标：两个程序都能与资源文件链接；`FileVersionInfo` 读出 0.1.2 和名称；Windows 能在 16、24、32、48 和 256 像素下取出图标；窗口的标题栏和页面头部都显示它。

**7. 0.1.3 中没有改变任何真实设备的检查（2026-11-24）。**

- 双击：Explorer 不带参数启动了 `release\bluelatch.exe`（真实 adapter，没有点击任何东西），同时每 2 毫秒轮询一次可见的顶层窗口。一个 Windows Terminal 窗口存在了 30 毫秒（启动后 372 毫秒到 402 毫秒），之后只剩 "BlueLatch" 窗口，在观察的 4.5 秒内一直存在。只有一个进程；关闭窗口后进程结束，日志里有 "starting the window" 和 "the window was closed"。
- 带参数 `window -demo` 的快捷方式，以同样的方式启动：结果相同，是 31 毫秒，窗口是 "BlueLatch (demo)"。
- 在终端里输入（工具的 PowerShell）：`bluelatch window -demo` 在 44 毫秒后返回，退出码 0，"BlueLatch (demo)" 窗口是唯一新出现的窗口，没有终端窗口。第二次 `bluelatch window -demo` 在 53 毫秒后返回，仍只有一个进程：已打开的窗口被带到了前台。通过 `cmd /c` 运行 `bluelatch window -demo`，64 毫秒后返回。
- 命令行仍然是控制台程序：通过 `cmd /v:on /c` 运行不带名称的 `bluelatch disconnect`，errorlevel 为 2。在 PowerShell 里 `$LASTEXITCODE` 同样是 2，不带名称的 `connect`、未知命令，以及 `window -lang fr` 也是如此，后者把错误和用法写到了 stderr。`window -h` 打印用法，退出码 0，`version` 和 `help` 正常，`list` 列出了设备。
- 这一个程序的窗口，用 `PrintWindow` 画成图片检查（用 `-demo-disconnect "Demo Speaker"` 启动）：带图标的标题栏、带 logo 的头部、三个演示设备、音箱上的 "Disconnected. The device stays paired."，以及页脚的 "v0.1.3"。
- `release\bluelatch.exe` 的 `FileVersionInfo` 读出 0.1.3、产品 "BlueLatch"、描述 "BlueLatch"、原始文件名 `bluelatch.exe`。PE 头中的子系统是 3（控制台）。
- 单元测试用假对象覆盖了控制台的三种状态，并真实地测试了 `StartDetached`：测试程序再启动自己一次，新进程报告控制台进程数为 0，并且带有变量 `BLUELATCH_DETACHED`。

**8. 切换声音输出（会改变 Windows 的设置；只能与用户一起做）。**

1. 选一个没有重要声音在播放的时候。用检查 1 的输出测试记下每个角色的默认输出。
2. 从**未提权**的进程运行 `bluelatch switch -v`（只有两个输出时，就切到另一个）。
3. 用输出测试检查 console 和 multimedia 角色已经移动，通话原本在旧输出上时也移动了。
4. 用 `bluelatch switch -v "<第一个输出的名称>"` 切回，检查三个角色都和开始时一样。
5. 记录日志中的 "changed the default output" 行以及输出显示的内容。

结果（2026-11-28），经用户许可：从未提权的进程，`switch` 把三个角色从蓝牙音箱换到显示器（"Switched the sound output from ... to ..."，退出码 0），输出测试显示三个角色都在显示器上；随后用音箱名称的 `switch` 又把三个角色换了回来。两次切换相隔不到一秒。之后用当前输出的名称执行 `switch`，打印 "is already the sound output."，没有改变任何东西；匹配不到的名称以退出码 1 失败，并列出所有输出。尚未做的：窗口的 **Switch** 按钮对真实输出的效果、通话在单独输出上的情况，以及带 Hands-Free 的耳机。

**9. 0.1.4 中没有改变任何真实设备和真实输出的检查（2026-11-28）。**

- 窗口演示模式（中文）：两个虚构输出，音箱打勾；在显示器上点 **Switch** 对勾移过去，再移回；对 Demo Speaker 执行 **Disconnect** 后它的输出消失，显示器成为当前输出；Demo Earbuds 连上后（3 次尝试、34.0 秒）它的输出出现。日志中每次切换都有 "changed the default output"。
- 窗口使用真实 adapter，只读，没有点击任何东西：列出了两个真实输出，音箱打勾，较长的输出名称显示在一行内。
- 在无头 Chrome 中用模拟后端查看页面：深色和浅色主题、英文和中文、切换进行中、切换失败、输出无法读取、没有输出，以及一个本身是 HTML 且含有 `{name}` 的输出名称，它按纯文本显示。

### 日志文件

`%LocalAppData%\BlueLatch\bluelatch.log`，由程序写入。程序启动时，如果超过 1 MiB，会被移到 `bluelatch.log.1`。`connect` 在第一行打印它的路径。设备连不上时请把日志发来：它记录了每次请求及其 Windows 错误码。

### 故障排查

| 现象 | 原因与处理 |
|---|---|
| `no Bluetooth radio found (is Bluetooth turned on?)`，或窗口中的红色横幅 | 电脑没有适配器，或蓝牙已关闭（预期如此，尚未验证）。在 Windows 设置中打开蓝牙。 |
| `the device is not paired` | Windows 没有该设备的配对，例如它在设置中被删除了。在 Windows 设置中重新配对。 |
| `the device has no installed audio service` | Windows 没有为该设备安装 A2DP Sink、Hands-Free 或 Headset 中的任何一个。如果 BlueLatch 曾在一个请求进行到一半时被停止，启动一次 `bluelatch connect` 或窗口：它会把服务重新打开（ADR 0005）。否则在 Windows 设置中删除该设备并重新配对。 |
| `Switched on the ... service of ... again; an earlier run had left it switched off.` | 不是问题：之前的一次运行在服务关闭和打开之间被停止了，这次启动修复了它。 |
| `No paired Bluetooth audio devices found`，或窗口为空 | 确实没有，或者设备是蓝牙 LE 设备，它们不会被列出（ADR 0002）。`list -all` 显示 Windows 报告的每个经典设备。 |
| `... is paired but is not an audio device` | `connect` 和 `disconnect` 只处理音频设备（ADR 0002）。 |
| `... is not connected.`（来自 `disconnect`） | 无需处理：该设备本来就没有连接。 |
| `... still reports being connected ... It may have connected again by itself.` | 断开请求已被接受，但设备仍显示为已连接，或立刻又连上了。有些设备被断开后会自己重新连接（ADR 0007）。把设备关机，或让它连到别的设备上，然后再试。 |
| `Windows answered (... enable failed: ... (error N))` | Windows 拒绝了请求。实际遇到的错误码的含义，将记录在这张表里。 |
| 关于 WebView2 运行时的消息框 | 窗口需要 Microsoft Edge WebView2 运行时，从消息中给出的页面获取。 |
| 窗口没有再打开第二个 | 已经有一个窗口打开了，并被带到了前台。 |
| 关闭窗口要几秒钟 | Windows 正在处理的请求要先结束（ADR 0005）。 |
| 窗口打开时有一个控制台或终端窗口闪了一下 | 控制台程序被双击或通过快捷方式启动时，Windows 会为它创建一个控制台，程序随即把它关闭：使用 Windows Terminal 时约 30 毫秒（ADR 0008）。从终端启动则没有闪烁。 |
| `bluelatch` 打开了窗口，而不是打印用法 | 从 0.1.3 起，不带命令的 `bluelatch` 打开窗口。用法请用 `bluelatch help`。 |
| `bluelatch: the window could not be started: ...` | 从终端启动时，程序无法为窗口再启动自己一次。文本里说明了原因。双击则在同一个进程中打开窗口。 |
| 在终端里输入 `bluelatch` 之后没有出现窗口 | 窗口在另一个进程中打开，所以那里的问题不会显示在终端里：它会显示在消息框里，并写入日志文件。 |
| `no sound output matches ...` | 该名称不在当前可用的输出里；消息中列出了它们。未连接的蓝牙音箱不算输出。 |
| `bluelatch switch: there is no other sound output to switch to` | 只有一个可用输出。连接或插入另一个。 |
| `Windows did not change the default output`（窗口中为 "Windows 没有切换输出。"） | Windows 接受了改变，但默认输出仍是别的。把日志发来。Windows 的声音弹出面板仍然可用。 |
| `Windows does not offer a way to change the default output: ...` | `IPolicyConfig` 接口不存在，例如在 Windows 更新之后（ADR 0009）。请使用 Windows 的声音弹出面板，并把带 HRESULT 的日志发来。 |
| `making ... the default output (...): IPolicyConfig.SetDefaultEndpoint: ... (HRESULT 0x...)` | Windows 拒绝了改变。实际遇到的错误码的含义，将记录在这张表里。 |
| 切换后声音没有跟过去 | 某个应用在音量混合器里被单独指定了输出，或者它要重启后才换到新输出。这取决于应用（未试过）。 |

### 发布

尚未建立。
