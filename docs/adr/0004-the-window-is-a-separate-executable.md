# English

## ADR 0004: The window is a separate executable

Status: accepted on 2026-11-24. Built in version 0.1.1. It resolves the open point of [ADR 0003](./0003-webview2-window.md).

### Context

BlueLatch has a command line (`bluelatch list`, `bluelatch connect`) and a window. A Windows program is built either for the console subsystem or for the GUI subsystem, and that decides how it behaves when it is started.

Options:

1. One executable for the console subsystem that also opens the window. A console window is created before the program can do anything about it, so double-clicking it flashes a console window. As far as we know, where Windows Terminal is the default terminal, it opens a terminal window instead.
2. One executable for the GUI subsystem that attaches to the console of its parent when it is started with a command. As far as we know, `cmd.exe` and PowerShell do not wait for such a program, so the prompt returns while the output is still being written, and Ctrl+C does not behave as usual.
3. Two executables built from the same module: `bluelatch.exe` for the console, `bluelatch-gui.exe` for the GUI subsystem.

### Decision

Use option 3. The command line keeps its name and its contract (AGENTS.md section 6). The window is `bluelatch-gui.exe`; it opens no console window.

### Consequences

- `release\` holds two files, built together by `scripts\build.ps1` with the same version.
- Both are thin: the logic is in `internal/`, so there is no duplicate code.
- The window is started by a double click or a shortcut. There is no `bluelatch gui` command.
- Neither prints to the other's console. The window shows problems in a message box and writes them to the log file.

---

# 中文

## ADR 0004：窗口是独立的可执行文件

状态：2026-11-24 已接受。已在 0.1.1 版中构建。它解决了 [ADR 0003](./0003-webview2-window.md) 中遗留的问题。

### 背景

BlueLatch 有命令行（`bluelatch list`、`bluelatch connect`）和窗口。Windows 程序要么按控制台子系统构建，要么按 GUI 子系统构建，这决定了它启动时的行为。

备选方案：

1. 一个控制台子系统的可执行文件，同时也打开窗口。控制台窗口在程序能做任何事之前就已创建，所以双击它会闪出一个控制台窗口。据我们所知，在 Windows Terminal 是默认终端的系统上，它会打开一个终端窗口。
2. 一个 GUI 子系统的可执行文件，在带命令启动时附加到父进程的控制台。据我们所知，`cmd.exe` 和 PowerShell 不会等待这样的程序，所以输出还在写的时候提示符就已经返回，Ctrl+C 的行为也与平常不同。
3. 由同一个模块构建的两个可执行文件：控制台用的 `bluelatch.exe`，GUI 子系统用的 `bluelatch-gui.exe`。

### 决定

采用方案 3。命令行保持原名和原有约定（AGENTS.md 第 6 节）。窗口是 `bluelatch-gui.exe`，它不会打开控制台窗口。

### 后果

- `release\` 中有两个文件，由 `scripts\build.ps1` 以相同版本一起构建。
- 两者都很薄：逻辑在 `internal/` 中，所以没有重复代码。
- 窗口通过双击或快捷方式启动。没有 `bluelatch gui` 命令。
- 两者都不会向对方的控制台输出。窗口把问题显示在消息框里，并写入日志文件。
