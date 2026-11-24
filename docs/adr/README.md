# English

## Architecture decision records

An ADR records a decision that was weighed between alternatives, and what follows from it. An ADR is not edited to change a decision: a later ADR supersedes it. Its status line and its observations are kept up to date.

| ADR | Decision | Status |
|---|---|---|
| [0001](./0001-reconnect-by-switching-audio-services.md) | A device is reconnected by switching its audio services off and on again through the Win32 Bluetooth API | Accepted; verified on one speaker, not yet on earbuds with Hands-Free |
| [0002](./0002-version-1-covers-classic-bluetooth-audio.md) | Version 1 handles classic Bluetooth audio devices only | Accepted |
| [0003](./0003-webview2-window.md) | The window is a WebView2 window with embedded HTML, through `go-webview2`, which needs no cgo | Accepted; built in 0.1.1 |
| [0004](./0004-the-window-is-a-separate-executable.md) | The window is a separate executable, `bluelatch-gui.exe`, next to the command line `bluelatch.exe` | Accepted; built in 0.1.1 |
| [0005](./0005-journal-of-service-switches.md) | A journal lets a later run switch on a service that was left switched off | Accepted; built in 0.1.1 |
| [0006](./0006-icon-snap-on-a-dark-tile.md) | The icon is Snap on a dark tile, embedded through a resource file that `tools/mkrsrc` writes | Accepted; built in 0.1.2 |
| [0007](./0007-disconnect-drops-the-link.md) | Disconnect drops the connection and leaves everything else alone | Accepted; built in 0.1.2; not tried on a connected device yet |

---

# 中文

## 架构决策记录

ADR 记录在多个备选方案之间权衡过的决定，以及由此带来的后果。不会通过修改 ADR 来改变决定：由后来的 ADR 取代它。它的状态行和观察到的情况会保持更新。

| ADR | 决定 | 状态 |
|---|---|---|
| [0001](./0001-reconnect-by-switching-audio-services.md) | 通过 Win32 蓝牙 API 把设备的音频服务先关闭再打开，来重新连接设备 | 已接受；已在一台音箱上验证，尚未在带 Hands-Free 的耳塞上验证 |
| [0002](./0002-version-1-covers-classic-bluetooth-audio.md) | 第 1 版只处理经典蓝牙音频设备 | 已接受 |
| [0003](./0003-webview2-window.md) | 窗口是内嵌 HTML 的 WebView2 窗口，通过不需要 cgo 的 `go-webview2` | 已接受；已在 0.1.1 中构建 |
| [0004](./0004-the-window-is-a-separate-executable.md) | 窗口是独立的可执行文件 `bluelatch-gui.exe`，与命令行 `bluelatch.exe` 并存 | 已接受；已在 0.1.1 中构建 |
| [0005](./0005-journal-of-service-switches.md) | 用待恢复记录让后续运行重新打开被遗留关闭的服务 | 已接受；已在 0.1.1 中构建 |
| [0006](./0006-icon-snap-on-a-dark-tile.md) | 图标是深色底的 Snap，通过 `tools/mkrsrc` 生成的资源文件嵌入 | 已接受；已在 0.1.2 中构建 |
| [0007](./0007-disconnect-drops-the-link.md) | Disconnect 只断开连接，不动其他任何东西 | 已接受；已在 0.1.2 中构建；尚未在已连接的设备上试过 |
