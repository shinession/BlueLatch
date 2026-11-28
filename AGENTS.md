# Project Engineering Instructions

These instructions apply to all AI-assisted development in this repository.

本说明适用于本代码仓库中的所有 AI 辅助开发工作。

---

# English

## 1. General Principles

Prefer correctness, maintainability, compatibility, and clarity over clever solutions.

Before changing existing behavior, understand how the current implementation works.

Reuse existing architecture, patterns, utilities, and conventions whenever possible.

Keep changes focused on the requested task. Avoid unnecessary refactoring, speculative redesign, and unrelated cleanup.

Do not claim that a task, test, verification step, or hardware check was completed unless it was actually performed.

When a larger change or a new feature has open design questions, discuss them with the user first, with a recommendation for each, and implement after the user has answered.

## 2. Architecture

BlueLatch is a Go program for Windows. It has these packages:

- `internal/bt`: the Windows Bluetooth stack behind the `Adapter` interface. The platform-independent types and the interface are in the files without a `_windows` suffix. The Win32 calls (`bthprops.cpl`, through `golang.org/x/sys/windows`, no cgo) are in the `*_windows.go` files.
- `internal/latch`: the retry engine. It knows nothing about Windows and talks to `bt.Adapter` only, so it is tested with a fake adapter and a fake clock.
- `internal/audio`: the sound outputs of Windows behind the `audio.Adapter` interface (ADR 0009). The types, the interface and the rule for switching the default output (`audio.Switch`) are in the files without a `_windows` suffix, so the rule is tested with a fake adapter. The Core Audio COM calls are in the `*_windows.go` files, through `golang.org/x/sys/windows`, no cgo.
- `internal/appdir`: the per-user folder `%LocalAppData%\BlueLatch`.
- `internal/logging`: the log file.
- `internal/app`: what the window shows and does: the list of paired audio devices, the sound outputs, at most one job (a connection or a disconnection), and at most one change of the output. It uses the engine and `internal/audio`, and the window uses it.
- `internal/ui`: the WebView2 window and its embedded page (`web/index.html`). A thin layer over `internal/app`.
- `cmd/bluelatch`: the program, one console program (ADR 0008). It is the command line, and it starts the window (`window.go`, `window_windows.go`, `console_windows.go`: how the process is attached to a console, the flags, then `ui.Run`).
- `assets/logo`: the icon, the SVG source and its PNG frames (ADR 0006).
- `tools/mkrsrc`: a development tool, not part of the program. It writes the resource files of the program (the icon and the version information) with the standard library only (ADR 0006).

Do not put retry logic in a user interface. Do not call the Win32 Bluetooth API outside `internal/bt`, do not call the Core Audio API outside `internal/audio`, and do not call window APIs outside `internal/ui`. The one exception is the console of the program: its Win32 calls are in `cmd/bluelatch/console_windows.go`. Do not introduce a second framework for a responsibility that one of these packages already covers.

If a major architectural change is necessary, explain why the existing architecture is insufficient before implementing the change.

## 3. Code Changes

Make the smallest complete change that satisfies the requirement.

Do not modify unrelated files. Do not rename commands, flags, or exported identifiers without a clear requirement.

Do not leave temporary debug code, temporary files, commented-out obsolete implementations, or unused experimental code.

Before modifying shared code, inspect the call sites and assess the regression risk.

## 4. Configuration

Do not hard-code real device names, device addresses, machine-specific paths, or credentials in source code, tests, or documentation examples. Use made-up names and addresses.

The program has no configuration file. Its tunable values are the command-line flags and `latch.Policy`. The log file is `%LocalAppData%\BlueLatch\bluelatch.log`.

## 5. Bluetooth and Device Safety

BlueLatch changes the state of the user's real Bluetooth devices, and the default sound output of the user's Windows.

- Never remove a pairing and never change anything of a device other than switching its audio services off and on again (`Reconnect`) and dropping its connection (`Disconnect`).
- A service that has been switched off must always be switched on again. `Reconnect` enables each service even when disabling it failed, and the engine never interrupts it.
- Every switch is recorded in the journal before it is made (`internal/bt/journal.go`, ADR 0005), and closing the window waits for a request in progress. Keep both when changing `Reconnect` or the window.
- `Disconnect` only drops the connection (ADR 0007). Do not disconnect a device by switching its services off or by removing its pairing: a service left off has to be repaired through the journal, and the program cannot restore a removed pairing.
- Do not run `bluelatch connect`, `bluelatch disconnect`, or any other code that calls `Reconnect` or `Disconnect`, against the user's devices unless the user asked for that test. A device may be in use, for example a speaker that is playing, and a disconnection interrupts it. Reading (`list`, `Devices`, `Device`) is always safe.
- Change the default sound output only when the user asks for it, with **Switch** or `bluelatch switch`, never on the program's own initiative, for example not after a connection (ADR 0009). Do not run `bluelatch switch`, or any other code that calls `SetDefault`, against the real outputs unless the user asked for that test, and put the previous defaults back afterwards. Reading the outputs (`State`) is always safe.
- Keep the program usable without administrator rights. If a change would need them, ask first. Test it from a process that is not elevated: a shell that is elevated hides the difference.
- The scope of version 1 is classic Bluetooth audio devices (ADR 0002). Bluetooth LE devices are not listed by the Win32 API in use and are out of scope.

## 6. User Interfaces

The command line is a contract. Command names, flags, output formats, and exit codes (0 success, 1 failure, 2 usage error) must not change without a clear requirement, because users put them into shortcuts and scripts. Human output goes to stdout, errors and warnings go to stderr.

`bluelatch` without a command opens the window, and `bluelatch help` prints the usage (ADR 0008). The program must stay a console program and must never be built with `-H=windowsgui`: a shell has to wait for it and see its exit code.

A window must show the same states the engine reports (`latch.Phase`) and must not invent its own retry behavior.

## 7. Security

Never place secrets in source code. The program sends nothing over the network and must stay that way.

Treat device names as untrusted text, because any nearby device can choose its name. Do not use them as format strings, file names, shell arguments, or HTML. When the window is built, it loads only content embedded in the executable.

## 8. Error Handling and Logging

Do not silently ignore errors. Handle them at the appropriate layer.

Every connection attempt is logged with the Windows error codes in the log file (`internal/logging`), because a connection that will not come up is diagnosed from that log. Log messages are in English only.

Do not log anything that is not needed to diagnose a failure.

## 9. Testing and Verification

Changes should be verified proportionally to their risk. Prefer this order:

1. targeted tests
2. `go test ./... -count=1`
3. `go vet ./...` and `gofmt -l .` (the last prints nothing when all is formatted)
4. a live check on a real device

`go test -race` does not work, because the project builds without cgo.

Tests that need Bluetooth hardware are skipped unless `BLUELATCH_LIVE=1` is set, and they must be read-only.

A change to the connection logic is not verified until it has been tried on a real device, and a change to how the output is switched until it has been tried on real outputs. Say so when it has not. A change to the window or its page is checked in demo mode (`bluelatch window -demo`), which touches no real device and no real output.

Do not modify or weaken tests only to hide a real implementation failure. Do not claim that a test passed unless it was actually executed and passed. If verification cannot be completed, clearly state what was not verified and why.

## 10. Dependency Management

Use the standard library and `golang.org/x/sys/windows`. Do not add a dependency when they already provide the functionality.

The window uses `github.com/jchv/go-webview2` (ADR 0003), which brings `github.com/jchv/go-winloader`. Another dependency needs a reason, a maintained package, and a recorded version.

The icon and the version information of the program are in the `cmd/bluelatch/rsrc_windows_*.syso` files, which `tools/mkrsrc` writes with the standard library only (ADR 0006). The files are committed. When the icon or the version changes, write them again with `scripts\resources.ps1`; do not add a third-party resource tool.

Do not upgrade unrelated dependencies unless necessary for the requested task.

## 11. Documentation Language Requirements

All documentation created or modified by AI agents MUST be bilingual in English and Simplified Chinese.

- English MUST always appear first: the complete English version, then the complete Simplified Chinese version. Do not place Chinese before English.
- This applies to every documentation file: README files, design documents, ADRs, runbooks, release notes, change logs, plans, and test plans.
- The Chinese version MUST mirror the structure and the technical content of the English version. Do not omit information from either language, simplify one language while keeping the other detailed, or change requirements, code examples, or technical identifiers between the versions.
- Technical identifiers normally stay unchanged: product names, file names, directory paths, command names, flag names, function and type names, and environment variables.
- Do not duplicate identical code blocks for translation. Write the English explanation, then the Chinese explanation, then one shared code block. Always close fenced code blocks correctly.
- Small tables may carry both languages in one table (`English / 中文`). For large tables, use a separate English table first and a separate Chinese table second.
- When modifying an existing document, keep both languages synchronized and keep English first.
- The requirement does NOT apply to source code, identifiers, log messages, error messages, Git commit messages, and inline source-code comments.

## 12. Documentation Quality

Documentation must describe the actual implementation, not an assumed implementation. Before documenting behavior, inspect the implementation.

Do not document features as completed if they are only planned. Do not invent paths, commands, flags, or behavior that do not exist. Clearly distinguish current behavior, planned behavior, assumptions, and what has been verified on a real device.

Write each kind of documentation where it belongs:

- `AGENTS.md` records only engineering rules and lessons that apply to the whole project.
- Documentation of a module or a topic goes to its own file in `docs/`, together with the lessons learned about it. Do not add it to `AGENTS.md`.
- `docs/README.md` is the index of those files. Look a topic up there before changing or debugging it, and add every new document to it.
- A decision that was weighed between alternatives is an ADR in `docs/adr/`, listed in `docs/adr/README.md`.

## 13. Completion

A task is not complete simply because code was generated. Completion requires, where applicable: implementation, validation, regression consideration, cleanup of temporary artifacts, synchronized documentation, and clear reporting of unresolved issues.

Before finishing, review the final changes for accidental edits, debugging leftovers, hard-coded values, missing error handling, missing tests, and documentation inconsistency.

## 14. Known Pitfalls

Lessons learned from real defects in this project. Check them before debugging a similar symptom. Add new entries as 14.1, 14.2, and so on, in both languages. Lessons about a single topic belong in that topic's document in `docs/`, not here.

There are no entries yet.

## 15. Git Commits and GitHub Sync

Commit only when the user explicitly asks for a commit in that request. Do not commit on your own initiative, when a task is finished, or because an earlier commit in the same session was requested. Without such a request, leave the changes in the working tree and say that they are not committed.

When the user asks for a commit, sync it with GitHub as part of the same request: fetch the remote, integrate any new remote commits (rebase, so that the history stays linear), then push the current branch to `origin`, and report the commit hash and the result of the push. If the sync cannot be completed (a conflict, a rejected push, no access, a history that has diverged), stop, report what happened and ask; never force-push, and never push a branch other than the one that was committed to.

---

# 中文

## 1. 通用原则

优先考虑正确性、可维护性、兼容性和清晰度，而不是追求过度复杂或炫技式的解决方案。

在修改现有行为之前，先理解当前实现的工作方式。

尽可能复用现有架构、模式、工具和项目约定。

修改范围应聚焦于当前任务。避免不必要的重构、推测性的重新设计以及与任务无关的清理工作。

除非实际已经执行，否则不得声称某项任务、测试、验证步骤或真机检查已经完成。

较大的改动或新功能存在未决的设计问题时，先与用户讨论，每个问题都给出推荐方案，用户回答之后再实施。

## 2. 架构

BlueLatch 是一个 Windows 下的 Go 程序，包含以下包：

- `internal/bt`：隐藏在 `Adapter` 接口后面的 Windows 蓝牙栈。与平台无关的类型和接口放在没有 `_windows` 后缀的文件中。Win32 调用（通过 `golang.org/x/sys/windows` 调用 `bthprops.cpl`，不使用 cgo）放在 `*_windows.go` 文件中。
- `internal/latch`：重试引擎。它不了解 Windows，只通过 `bt.Adapter` 工作，所以用假的 adapter 和假的时钟来测试。
- `internal/audio`：隐藏在 `audio.Adapter` 接口后面的 Windows 声音输出（ADR 0009）。类型、接口和切换默认输出的规则（`audio.Switch`）放在没有 `_windows` 后缀的文件中，所以规则用假的 adapter 来测试。Core Audio 的 COM 调用放在 `*_windows.go` 文件中，通过 `golang.org/x/sys/windows`，不使用 cgo。
- `internal/appdir`：每个用户的文件夹 `%LocalAppData%\BlueLatch`。
- `internal/logging`：日志文件。
- `internal/app`：窗口显示和执行的内容：已配对音频设备的列表、声音输出、至多一个任务（连接或断开），以及至多一次输出切换。它使用引擎和 `internal/audio`，窗口使用它。
- `internal/ui`：WebView2 窗口及其嵌入的页面（`web/index.html`）。`internal/app` 之上的薄层。
- `cmd/bluelatch`：程序本身，一个控制台程序（ADR 0008）。它是命令行，并负责启动窗口（`window.go`、`window_windows.go`、`console_windows.go`：进程如何附着在控制台上、参数，然后调用 `ui.Run`）。
- `assets/logo`：图标，SVG 源文件及其 PNG 帧（ADR 0006）。
- `tools/mkrsrc`：开发工具，不属于程序本身。它只用标准库写出程序的资源文件（图标和版本信息）（ADR 0006）。

不要把重试逻辑放进用户界面。不要在 `internal/bt` 之外调用 Win32 蓝牙 API，不要在 `internal/audio` 之外调用 Core Audio API，也不要在 `internal/ui` 之外调用窗口 API。唯一的例外是程序自己的控制台：它的 Win32 调用在 `cmd/bluelatch/console_windows.go` 中。不要为这些包已经覆盖的职责再引入第二套框架。

如果确实需要较大的架构调整，应在实施前说明现有架构为什么不够用。

## 3. 代码修改

采用能够完整满足需求的最小修改范围。

不要修改与任务无关的文件。除非需求明确要求，否则不要重命名命令、参数或导出的标识符。

不要遗留临时调试代码、临时文件、已废弃的注释代码或未使用的实验代码。

修改共享代码前，应检查调用位置并评估回归风险。

## 4. 配置

不要在源代码、测试或文档示例中硬编码真实的设备名称、设备地址、特定机器的路径或凭据。使用虚构的名称和地址。

程序没有配置文件。可调的值是命令行参数和 `latch.Policy`。日志文件是 `%LocalAppData%\BlueLatch\bluelatch.log`。

## 5. 蓝牙与设备安全

BlueLatch 会改变用户真实蓝牙设备的状态，以及用户 Windows 的默认声音输出。

- 绝不要删除配对，也不要改动设备的其他任何内容，只允许把它的音频服务先关闭再打开（`Reconnect`），以及断开它的连接（`Disconnect`）。
- 被关闭的服务必须总是被重新打开。`Reconnect` 即使在关闭失败时也会启用每个服务，引擎也绝不会中断它。
- 每次开关在进行之前都会记录到待恢复记录中（`internal/bt/journal.go`，ADR 0005），关闭窗口会等待进行中的请求。修改 `Reconnect` 或窗口时要保留这两点。
- `Disconnect` 只断开连接（ADR 0007）。不要通过关闭设备的服务或删除它的配对来断开设备：被遗留关闭的服务要靠待恢复记录来修复，被删除的配对程序无法恢复。
- 除非用户要求做这个测试，否则不要对用户的设备运行 `bluelatch connect`、`bluelatch disconnect`，或任何其他会调用 `Reconnect` 或 `Disconnect` 的代码。设备可能正在使用，例如正在播放的音箱，断开会打断它。读取（`list`、`Devices`、`Device`）始终是安全的。
- 只有在用户用 **Switch** 或 `bluelatch switch` 提出要求时才改变默认声音输出，绝不由程序自行改变，例如不在连接成功之后改变（ADR 0009）。除非用户要求做这个测试，否则不要对真实输出运行 `bluelatch switch`，或任何其他会调用 `SetDefault` 的代码；测试之后要恢复原来的默认项。读取输出（`State`）始终是安全的。
- 保持程序在没有管理员权限的情况下可用。如果某个改动需要管理员权限，先询问。要在未提权的进程中测试：已提权的 shell 会掩盖这种差别。
- 第 1 版的范围是经典蓝牙音频设备（ADR 0002）。所用的 Win32 API 不会列出蓝牙 LE 设备，它们不在范围内。

## 6. 用户界面

命令行是一份约定。命令名、参数、输出格式和退出码（0 成功，1 失败，2 用法错误）在没有明确需求时不得改变，因为用户会把它们写进快捷方式和脚本。面向人的输出写到 stdout，错误和警告写到 stderr。

不带命令的 `bluelatch` 打开窗口，`bluelatch help` 打印用法（ADR 0008）。程序必须保持为控制台程序，绝不能用 `-H=windowsgui` 构建：shell 必须能等待它并拿到它的退出码。

窗口必须显示引擎报告的同一组状态（`latch.Phase`），不得自行发明重试行为。

## 7. 安全

绝不要把机密信息写入源代码。程序不通过网络发送任何内容，必须保持这一点。

把设备名称视为不可信文本，因为附近任何设备都可以自己选择名称。不要把它们用作格式字符串、文件名、shell 参数或 HTML。构建窗口时，它只加载嵌入在可执行文件中的内容。

## 8. 错误处理与日志

不要静默忽略错误。应在正确的层级处理错误。

每次连接尝试都会连同 Windows 错误码记录到日志文件中（`internal/logging`），因为连不上的问题要靠这份日志来诊断。日志消息只用英文。

不要记录诊断失败所不需要的内容。

## 9. 测试与验证

修改应按照风险程度进行相应验证。推荐顺序：

1. 针对性测试
2. `go test ./... -count=1`
3. `go vet ./...` 和 `gofmt -l .`（全部已格式化时后者没有任何输出）
4. 在真实设备上的真机检查

`go test -race` 无法使用，因为项目不使用 cgo 构建。

需要蓝牙硬件的测试，除非设置了 `BLUELATCH_LIVE=1`，否则会被跳过，并且它们必须是只读的。

对连接逻辑的修改，在真实设备上试过之前都不算验证过；对输出切换方式的修改，在真实输出上试过之前也不算。没有试过时要明确说明。对窗口或其页面的修改，在演示模式（`bluelatch window -demo`）下检查，它不会触碰任何真实设备和真实输出。

不要仅仅为了让测试通过而修改或削弱测试，从而掩盖真实实现问题。除非测试实际执行并通过，否则不得声称测试已经通过。如果无法完成某项验证，应明确说明未验证的内容及原因。

## 10. 依赖管理

使用标准库和 `golang.org/x/sys/windows`。如果它们已经提供了所需功能，不要引入新的依赖。

窗口使用 `github.com/jchv/go-webview2`（ADR 0003），它会带入 `github.com/jchv/go-winloader`。再增加其他依赖，需要说明理由、选用维护活跃的包，并记录版本。

程序的图标和版本信息在 `cmd/bluelatch/rsrc_windows_*.syso` 文件中，由 `tools/mkrsrc` 只用标准库写出（ADR 0006）。这些文件已提交。图标或版本变化时，用 `scripts\resources.ps1` 重新写出它们；不要引入第三方的资源工具。

除非当前任务确实需要，否则不要升级无关依赖。

## 11. 文档语言要求

所有由 AI Agent 创建或修改的文档都必须同时提供英文和简体中文版本。

- 英文必须始终放在前面：先是完整英文版，再是完整简体中文版。不要把中文放在英文之前。
- 适用于所有文档文件：README 文件、设计文档、ADR、运行手册、发布说明、变更日志、计划和测试计划。
- 中文版必须与英文版保持相同的结构和技术内容。不要让任一语言遗漏内容，不要简化其中一种语言而保留另一种的详细内容，也不要在两个版本之间改变需求、代码示例或技术标识符。
- 技术标识符通常保持原样：产品名称、文件名、目录路径、命令名、参数名、函数名和类型名、环境变量。
- 不要仅为了翻译而重复完全相同的代码块。先写英文说明，再写中文说明，然后只提供一个共享代码块。必须正确关闭 fenced code block。
- 较小的表格可以把两种语言放在同一个表格中（`English / 中文`）。较大的表格，先放独立的英文表格，再放独立的中文表格。
- 修改现有文档时，保持两个语言版本同步，并保持英文在前。
- 此要求不适用于源代码、标识符、日志消息、错误消息、Git 提交信息和源代码中的行内注释。

## 12. 文档质量

文档必须描述实际实现，而不是假设中的实现。在记录行为之前，先检查实现。

如果某项功能仍处于计划阶段，不要把它写成已经完成。不要虚构不存在的路径、命令、参数或行为。必须清楚区分当前行为、计划行为、假设，以及已经在真实设备上验证过的内容。

文档要写在应该写的位置：

- `AGENTS.md` 只记录适用于整个项目的工程规则和经验。
- 单个模块或主题的文档写入 `docs/` 下它自己的文件，该主题的经验教训也写在同一个文件里，不要加到 `AGENTS.md`。
- `docs/README.md` 是这些文件的索引。修改或排查某个主题之前先在其中查找，新增文档时也要把它加到索引中。
- 在多个备选方案之间权衡过的决定，写成 `docs/adr/` 中的 ADR，并列入 `docs/adr/README.md`。

## 13. 完成标准

仅生成代码并不意味着任务已经完成。在适用情况下，完成标准包括：实现完成、验证完成、考虑回归风险、清理临时产物、文档保持同步，以及清楚报告未解决问题。

结束任务前，应检查最终修改是否存在意外变更、调试残留、硬编码值、缺失错误处理、缺失测试或中英文文档不一致的问题。

## 14. 已知问题与经验

来自本项目真实缺陷的经验。再次排查类似现象前先查阅。新增条目请按 14.1、14.2……编号，并同时更新中英文两个版本。只涉及单个主题的经验写入 `docs/` 中该主题的文档，不要写在这里。

目前还没有条目。

## 15. Git 提交与 GitHub 同步

只有在用户于该次请求中明确要求提交（commit）时才提交。不要自行决定提交，不要在任务完成时顺手提交，也不要因为同一会话中之前应要求提交过就再次提交。没有这样的要求时，把修改留在工作区，并说明它们尚未提交。

用户要求提交时，把同步到 GitHub 作为同一请求的一部分：先 fetch 远程，整合远程的新提交（用 rebase，让历史保持线性），再把当前分支 push 到 `origin`，并报告提交哈希和 push 的结果。如果同步无法完成（冲突、push 被拒绝、没有权限、历史已分叉），就停下来，说明发生了什么并询问用户；绝不 force-push，也绝不推送被提交的分支之外的其他分支。
