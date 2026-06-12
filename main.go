package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		if err := runServer(); err != nil {
			fmt.Fprintf(os.Stderr, "ERR 启动服务器失败: %v\n", err)
			os.Exit(1)
		}
		return
	}

	cmd := os.Args[1]

	var err error
	switch cmd {
	case "test":
		err = testCmd()

	case "set":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "ERR 缺少 DNS 名称参数")
			usage()
			os.Exit(1)
		}
		primary := os.Args[2]
		secondary := ""
		if len(os.Args) >= 4 {
			secondary = os.Args[3]
		}
		err = setCmd(primary, secondary)

	case "restore":
		err = restoreCmd()

	default:
		fmt.Fprintf(os.Stderr, "ERR 未知命令 %q\n", cmd)
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERR %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`用法:
  dns-switch                     启动网页管理面板 (http://127.0.0.1:9753)
  dns-switch test                 测速所有 DNS 服务器并显示延迟排行
  dns-switch set <主DNS> [备DNS]   切换到指定 DNS（可设主备）
  dns-switch restore              恢复网卡为 DHCP 自动获取`)
}
