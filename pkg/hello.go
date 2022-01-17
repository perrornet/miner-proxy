package pkg

import (
	"fmt"

	"github.com/common-nighthawk/go-figure"
)

func PrintHelp() {
	myFigure := figure.NewFigure("Miner Proxy", "", true)
	myFigure.Print()
	// 免责声明以及项目地址
	fmt.Println("项目地址: https://github.com/PerrorOne/miner-proxy")
	fmt.Println("免责声明: 本工具只适用于测试与学习使用, 请勿将其使用到挖矿活动上!!")
}
