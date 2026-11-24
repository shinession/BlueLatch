# English

## BlueLatch

BlueLatch connects a paired Bluetooth audio device on Windows and keeps retrying until the connection holds. It is for the device that shows "Not connected" in Windows Settings after the first time and needs several clicks on **Connect**.

### Status

Version 0.1.0 is a spike: a command line only. The window is decided (ADR 0003) but not built.

Verified on a real PC (Windows 11, Intel radio): listing the paired audio devices, in an elevated process and in a normal one; connecting a device that is connected already (nothing is touched).

**Not verified yet:** the reconnect request itself on a device that is out of reach, and whether it needs administrator rights. See the live check in [docs/runbook.md](docs/runbook.md).

### Requirements

- Windows with a Bluetooth radio (tried on Windows 11 only). The device must be paired in Windows Settings first.
- To build: Go 1.27 or newer. No C compiler is needed.

### Usage

```text
bluelatch list [-all] [-v]
bluelatch connect [-v] [-attempt-timeout 10s] [-stable 2s] [-max-attempts N] <name or address>
bluelatch version
```

`list` shows the paired audio devices with their state. `-all` also shows the other devices Windows reports, `-v` adds the Windows flags, the class, and the installed services.

`connect` finds the device by address or by name (a whole name, or a part of it that is unique; case is ignored). It asks Windows to connect the device, waits, and asks again until the device has stayed connected for `-stable`. Stop it with Ctrl+C. One attempt waits at most `-attempt-timeout` for the link. The pause between attempts grows from 2 s to 5 s. `-max-attempts` limits the attempts; the default 0 means no limit. A device that is connected already is reported at once and is not touched.

Exit codes: 0 success, 1 failure or stopped, 2 wrong command line.

### How it works

Windows has no documented call that simply connects a classic audio device. BlueLatch switches the audio services of the device (A2DP, Hands-Free, Headset) off and on again through the Win32 Bluetooth API, which asks Windows to connect them (ADR 0001). Then it reads the connection state until it holds. Every attempt is logged with the Windows error codes to `%LocalAppData%\BlueLatch\bluelatch.log`.

### Limits

- Classic Bluetooth audio devices only: headphones, earbuds, speakers (ADR 0002). Bluetooth LE devices, such as most mice and keyboards, are not listed.
- A device that is switched off, in its case, or out of range cannot be connected. BlueLatch keeps trying and connects as soon as the device is available.

### Build and test

```powershell
.\scripts\build.ps1 -Version 0.1.0   # writes release\bluelatch.exe
go test ./... -count=1
```

### Documentation

[docs/README.md](docs/README.md) is the index. The engineering rules for contributors and AI agents are in [AGENTS.md](AGENTS.md).

---

# 中文

## BlueLatch

BlueLatch 在 Windows 上连接已配对的蓝牙音频设备，并不断重试，直到连接稳定。它面向这样的设备：第一次之后在 Windows 设置里显示"未连接"，需要多次点击 **Connect**。

### 状态

0.1.0 版是一个验证原型（spike）：目前只有命令行。窗口已确定（ADR 0003），但尚未构建。

已在一台真实的电脑上验证（Windows 11，Intel 蓝牙适配器）：在已提权的进程和普通进程中列出已配对的音频设备；对已经连接的设备执行连接（不会触碰设备）。

**尚未验证：**对够不着的设备发出的重新连接请求本身，以及它是否需要管理员权限。参见 [docs/runbook.md](docs/runbook.md) 中的真机检查。

### 要求

- 带蓝牙适配器的 Windows（只在 Windows 11 上试过）。设备必须先在 Windows 设置中完成配对。
- 构建需要 Go 1.27 或更高版本，不需要 C 编译器。

### 用法

```text
bluelatch list [-all] [-v]
bluelatch connect [-v] [-attempt-timeout 10s] [-stable 2s] [-max-attempts N] <name or address>
bluelatch version
```

`list` 显示已配对的音频设备及其状态。`-all` 同时显示 Windows 报告的其他设备，`-v` 增加 Windows 标志、设备类别和已安装的服务。

`connect` 按地址或名称查找设备（完整名称，或者唯一的名称片段；不区分大小写）。它请求 Windows 连接设备，等待，再次请求，直到设备连续保持连接 `-stable` 这么久。用 Ctrl+C 停止。一次尝试最多等待 `-attempt-timeout` 让连接建立。两次尝试之间的间隔从 2 秒逐步增加到 5 秒。`-max-attempts` 限制尝试次数，默认的 0 表示不限制。已经连接的设备会立即报告，并且不会被触碰。

退出码：0 成功，1 失败或被停止，2 命令行有误。

### 工作原理

Windows 没有文档化的调用可以直接连接经典蓝牙音频设备。BlueLatch 通过 Win32 蓝牙 API 把设备的音频服务（A2DP、Hands-Free、Headset）先关闭再打开，这会请求 Windows 去连接它们（ADR 0001）。然后读取连接状态，直到连接稳定。每次尝试都会连同 Windows 错误码记录到 `%LocalAppData%\BlueLatch\bluelatch.log`。

### 限制

- 只支持经典蓝牙音频设备：耳机、耳塞、音箱（ADR 0002）。蓝牙 LE 设备（例如大多数鼠标和键盘）不会列出。
- 已关机、在充电盒里或不在范围内的设备无法连接。BlueLatch 会不断重试，设备一可用就会连上。

### 构建与测试

```powershell
.\scripts\build.ps1 -Version 0.1.0   # writes release\bluelatch.exe
go test ./... -count=1
```

### 文档

[docs/README.md](docs/README.md) 是索引。面向贡献者和 AI Agent 的工程规则在 [AGENTS.md](AGENTS.md) 中。
