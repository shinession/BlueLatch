# English

## ADR 0002: Version 1 handles classic Bluetooth audio devices only

Status: accepted on 2026-11-24, by the user.

### Context

In the Windows Settings page we looked at, only the earbuds had a **Connect** button; a Bluetooth LE mouse had none. Bluetooth LE devices (HID over GATT) normally reconnect by themselves when the device wakes up, and a PC cannot connect one that is asleep.

The Win32 API of ADR 0001 lists classic devices only. Verified on the development PC: a paired Bluetooth LE mouse is not listed, not even by `bluelatch list -all`. Supporting LE devices needs another API (WinRT or GATT), roughly twice the work, and there is no guarantee that it helps.

### Decision

Version 1 handles paired classic Bluetooth audio devices. A device counts as an audio device when its class is Audio/Video, or when Windows has installed A2DP Sink, Hands-Free, or Headset for it.

`list` shows the paired audio devices. `list -all` shows every classic device Windows reports. `connect` refuses a paired device that is not an audio device, with a message that says so.

### Consequences

- Bluetooth LE devices are out of scope and invisible to the program.
- Revisit this when a real Bluetooth LE device has the same problem.

---

# 中文

## ADR 0002：第 1 版只处理经典蓝牙音频设备

状态：2026-11-24 由用户接受。

### 背景

在我们看到的 Windows 设置页面里，只有耳塞有 **Connect** 按钮；一个蓝牙 LE 鼠标没有。蓝牙 LE 设备（HID over GATT）通常在设备唤醒后自行重新连接，电脑无法连接一个正在睡眠的设备。

ADR 0001 所用的 Win32 API 只列出经典设备。在开发用电脑上已验证：已配对的蓝牙 LE 鼠标不会被列出，连 `bluelatch list -all` 也不会。支持 LE 设备需要另一套 API（WinRT 或 GATT），工作量大约翻倍，并且不能保证有帮助。

### 决定

第 1 版处理已配对的经典蓝牙音频设备。设备的类别是 Audio/Video，或者 Windows 已为它安装了 A2DP Sink、Hands-Free 或 Headset，就算音频设备。

`list` 显示已配对的音频设备。`list -all` 显示 Windows 报告的每个经典设备。`connect` 会拒绝已配对但不是音频设备的设备，并给出说明。

### 后果

- 蓝牙 LE 设备不在范围内，程序也看不到它们。
- 当真实的蓝牙 LE 设备出现同样的问题时，再重新考虑这一点。
