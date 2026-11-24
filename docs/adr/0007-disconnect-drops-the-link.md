# English

## ADR 0007: Disconnect drops the connection and leaves everything else alone

Status: accepted on 2026-11-24, at the request of the user (version 1 had no Disconnect). Built in version 0.1.2. The user tried it on a connected speaker, and it worked (see Observed); it has not been tried on the earbuds.

### Context

The window and the command line should be able to disconnect a connected audio device, like the Disconnect command of Windows Settings, so that the device can be used with something else, or be connected again with BlueLatch.

Options:

1. Ask the radio to drop the baseband connection: `IOCTL_BTH_DISCONNECT_DEVICE` with the address of the device, sent with `DeviceIoControl` to the handle of the radio. The services and the pairing stay as they are. As far as we know, this is what the Disconnect of Windows Settings amounts to as well.
2. Switch the audio services off, as a reconnect does, and leave them off. It would keep the device from connecting again by itself, but a device whose services are off cannot be connected by Windows Settings or by this program until they are switched on, and the journal of ADR 0005 would take it for an interrupted request.
3. Remove the pairing (`BluetoothRemoveDevice`). Never: the device would have to be paired again.
4. Restart the device node, or turn Bluetooth off. It needs administrator rights and affects other devices.

### Decision

Use option 1.

`Engine.Disconnect` reads the device first. A device that is not connected is reported as such and nothing is sent. Otherwise it sends the request and waits up to 5 s (`Policy.DisconnectTimeout`) for Windows to report the device as disconnected, which is success. When the device still reports being connected after that, it says so (`ErrStillConnected`).

The window shows a Disconnect button on the card of a connected device. The command line has `bluelatch disconnect <name or address>`.

### Observed

On 2026-11-24 the request was sent as a normal user (integrity level Medium) for earbuds that were not connected. Windows answered with ERROR_DEVICE_NOT_CONNECTED (1167), not with an access error, so the request is not refused for lack of administrator rights. Only that code is taken for "not connected".

The same day, at 23:21, the user clicked Disconnect on the card of a connected speaker in the window of 0.1.2, and Connect on the same card about five seconds later. The log shows: the request accepted at once ("the disconnect request was accepted"); the speaker reported as disconnected at the first read of its state, 0.5 s later; then the Connect, which found the speaker not connected (so it had not connected again by itself within those five seconds), switched its A2DP Sink service off and on, and connected it on the first attempt after 8.4 s. The folder `pending` stayed empty. The log does not say whether the window had been started elevated.

### Consequences

- Nothing is switched off, so there is nothing to repair (no record in the journal) and nothing to finish when the program is stopped.
- A device may connect again by itself as soon as it is dropped. That is up to the device. The program then reports that the device still reports being connected, or the list shows it connected again.
- Only one job runs at a time. Disconnect is refused while a connection is being retried (the button is disabled), and Connect while a disconnection is running.
- Known from the run on the speaker: the request drops the link at once, Windows reports it at the first read after 0.5 s, and the speaker did not connect again by itself within about 5 s. Not known: how the earbuds (A2DP and Hands-Free) behave, and whether some devices connect again by themselves after they are dropped.

---

# 中文

## ADR 0007：Disconnect 只断开连接，不动其他任何东西

状态：2026-11-24 应用户要求接受（第 1 版没有 Disconnect）。已在 0.1.2 版中构建。用户在一台已连接的音箱上试过，可以工作（见"观察到的情况"）；尚未在耳塞上试过。

### 背景

窗口和命令行应该能断开已连接的音频设备，就像 Windows 设置里的 Disconnect 命令一样，这样设备可以用于别处，或者之后再用 BlueLatch 连接。

备选方案：

1. 请求无线电适配器断开基带连接：用 `DeviceIoControl` 向适配器的句柄发送 `IOCTL_BTH_DISCONNECT_DEVICE`，输入是设备的地址。服务和配对保持不变。据我们所知，Windows 设置里的 Disconnect 本质上也是这样。
2. 像重新连接那样把音频服务关闭，并保持关闭。这样能防止设备自己再连上，但服务关闭的设备，在服务重新打开之前，Windows 设置和本程序都无法连接它，而且 ADR 0005 的待恢复记录会把它当作被中断的请求。
3. 删除配对（`BluetoothRemoveDevice`）。绝不：设备需要重新配对。
4. 重启设备节点，或关闭蓝牙。需要管理员权限，还会影响其他设备。

### 决定

采用方案 1。

`Engine.Disconnect` 先读取设备。设备未连接时，如实报告，什么也不发送。否则发出请求，并最多等待 5 秒（`Policy.DisconnectTimeout`），等 Windows 报告设备已断开，这就是成功。如果之后设备仍显示为已连接，就明说（`ErrStillConnected`）。

窗口在已连接设备的卡片上显示 Disconnect 按钮。命令行有 `bluelatch disconnect <name or address>`。

### 观察到的情况

2026-11-24 以普通用户（完整性级别 Medium）对一副未连接的耳塞发送了该请求。Windows 返回 ERROR_DEVICE_NOT_CONNECTED（1167），而不是访问错误，所以该请求不会因为缺少管理员权限而被拒绝。只有这个错误码会被当作"未连接"。

同一天 23:21，用户在 0.1.2 的窗口里点击了一台已连接音箱卡片上的 Disconnect，约五秒后又点击了同一张卡片上的 Connect。日志显示：请求被立即接受（"the disconnect request was accepted"）；0.5 秒后第一次读取状态时，音箱已被报告为断开；随后的 Connect 发现音箱未连接（所以它在这五秒内没有自己再连上），把它的 A2DP Sink 服务先关闭再打开，第一次尝试就在 8.4 秒后连上了。文件夹 `pending` 始终为空。日志没有说明窗口是否以提权方式启动。

### 后果

- 没有任何东西被关闭，所以没有需要修复的（待恢复记录里没有记录），程序被停止时也没有需要完成的。
- 设备被断开后可能立刻自己又连上。这由设备决定。此时程序会报告设备仍显示为已连接，或者列表显示它又连上了。
- 同一时间只运行一个任务。重试连接期间拒绝 Disconnect（按钮被禁用），断开期间拒绝 Connect。
- 从音箱上的这次运行得知：请求会立刻断开链路，Windows 在 0.5 秒后的第一次读取时就报告了，音箱在约 5 秒内没有自己再连上。尚不清楚：耳塞（A2DP 加 Hands-Free）的表现，以及是否有些设备被断开后会自己再连上。
