# English

## Architecture decision records

An ADR records a decision that was weighed between alternatives, and what follows from it. An ADR is not edited to change a decision: a later ADR supersedes it.

| ADR | Decision | Status |
|---|---|---|
| [0001](./0001-reconnect-by-switching-audio-services.md) | A device is reconnected by switching its audio services off and on again through the Win32 Bluetooth API | Accepted; not verified yet on a device that is out of reach |
| [0002](./0002-version-1-covers-classic-bluetooth-audio.md) | Version 1 handles classic Bluetooth audio devices only | Accepted |
| [0003](./0003-webview2-window.md) | The window is a WebView2 window with embedded HTML, through a binding that needs no cgo | Accepted; not built yet |

---

# 中文

## 架构决策记录

ADR 记录在多个备选方案之间权衡过的决定，以及由此带来的后果。不会通过修改 ADR 来改变决定：由后来的 ADR 取代它。

| ADR | 决定 | 状态 |
|---|---|---|
| [0001](./0001-reconnect-by-switching-audio-services.md) | 通过 Win32 蓝牙 API 把设备的音频服务先关闭再打开，来重新连接设备 | 已接受；尚未在够不着的设备上验证 |
| [0002](./0002-version-1-covers-classic-bluetooth-audio.md) | 第 1 版只处理经典蓝牙音频设备 | 已接受 |
| [0003](./0003-webview2-window.md) | 窗口是内嵌 HTML 的 WebView2 窗口，使用不依赖 cgo 的绑定 | 已接受；尚未构建 |
