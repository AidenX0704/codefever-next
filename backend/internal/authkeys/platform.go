package authkeys

// Supported 报告当前平台是否支持写 authorized_keys。
//
// 容器内是 Linux（true）；Windows 开发机是 false —— 那里既没有
// /home/<git 用户>/.ssh，也没有 sshd，同步没有意义。
//
// 调用方（handlers）据此**跳过**同步而不是报错：这是"不阻断业务"取舍的一部分。
func Supported() bool { return supported }
