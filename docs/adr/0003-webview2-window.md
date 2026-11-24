# English

## ADR 0003: The window is a WebView2 window with embedded HTML

Status: accepted on 2026-11-24, by the user. Not built yet.

### Context

The program needs a window with the paired devices, a **Connect** button for each, and the progress of the retries. The development PC has no C compiler (cgo is off) and no Node. Windows Settings on it is in dark mode.

Options:

1. A WebView2 window showing HTML, CSS, and JavaScript embedded in the executable, through a Go binding that needs no cgo. The page follows the dark or light theme of the system with the CSS `prefers-color-scheme`, it is easy to change and to check with screenshots, and the executable stays small. It needs the WebView2 runtime, which is installed on the development PC (version 157.0.4341.60) and is part of Windows 11.
2. Native Win32 controls (walk). No runtime dependency, but as far as we know no dark mode: a bright window on a dark desktop.
3. A tray icon with a menu. The lightest, but it cannot show the progress of the retries.
4. Fyne needs a C compiler on Windows, and Wails needs Node for its front end. Neither is possible on this PC without installing more.

### Decision

Use option 1. The retry engine and `internal/bt` stay independent of the window, so the window can be replaced.

### Consequences

- The WebView2 runtime must be present. Windows 11 has it. Windows 10 may need it installed.
- A WebView2 window uses more memory than native controls, so it is not meant to stay resident in the tray.
- The binding is chosen when the window is built (AGENTS.md section 10), and this ADR is updated with the choice.
- Device names are set in the page as text, never as HTML (AGENTS.md section 7).
- If a command-line build and a window build must share one executable, the console subsystem has to be decided then. Not decided yet.

---

# 中文

## ADR 0003：窗口是内嵌 HTML 的 WebView2 窗口

状态：2026-11-24 由用户接受。尚未构建。

### 背景

程序需要一个窗口，显示已配对的设备，每个设备一个 **Connect** 按钮，并显示重试的进度。开发用电脑没有 C 编译器（cgo 关闭），也没有 Node。电脑上的 Windows 设置是深色模式。

备选方案：

1. WebView2 窗口，显示嵌入在可执行文件中的 HTML、CSS 和 JavaScript，通过不依赖 cgo 的 Go 绑定。页面用 CSS `prefers-color-scheme` 跟随系统的深色或浅色主题，容易修改，也容易用截图检查，可执行文件保持较小。它需要 WebView2 运行时，开发用电脑上已安装（版本 157.0.4341.60），并且它是 Windows 11 的一部分。
2. 原生 Win32 控件（walk）。没有运行时依赖，但据我们所知不支持深色模式：在深色桌面上是一个明亮的窗口。
3. 带菜单的托盘图标。最轻量，但无法显示重试的进度。
4. Fyne 在 Windows 上需要 C 编译器，Wails 的前端需要 Node。在这台电脑上，不再安装其他东西，两者都无法使用。

### 决定

采用方案 1。重试引擎和 `internal/bt` 保持独立于窗口，所以窗口可以被替换。

### 后果

- 必须有 WebView2 运行时。Windows 11 自带，Windows 10 可能需要安装。
- WebView2 窗口比原生控件占用更多内存，所以不适合常驻托盘。
- 构建窗口时再选定绑定（AGENTS.md 第 10 节），并把选择写回本 ADR。
- 设备名称在页面中以文本方式设置，绝不作为 HTML（AGENTS.md 第 7 节）。
- 如果命令行版本和窗口版本必须共用一个可执行文件，届时需要决定控制台子系统的问题。目前尚未决定。
