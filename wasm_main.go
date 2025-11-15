//go:build js && wasm
// +build js,wasm

package main

import (
	"fmt"
	"log"
	"net"
	"syscall/js"
	"time"

	"GoFileShare/services"
)

// WASMClient WASM客户端
type WASMClient struct {
	p2pManager *services.P2PConnectionManager
	isRunning  bool
}

var wasmClient *WASMClient

// 初始化WASM客户端
func initWASMClient() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) < 1 {
			return map[string]interface{}{
				"success": false,
				"error":   "缺少客户端ID参数",
			}
		}

		clientID := args[0].String()

		// 使用Google STUN服务器初始化P2P连接管理器
		stunServer := "stun.l.google.com:19302"
		manager, err := services.NewP2PConnectionManager(stunServer, clientID)
		if err != nil {
			return map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("初始化P2P管理器失败: %v", err),
			}
		}

		wasmClient = &WASMClient{
			p2pManager: manager,
			isRunning:  true,
		}

		// 获取连接信息
		localAddr, publicAddr := manager.GetConnectionInfo()

		return map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"clientId":   clientID,
				"localAddr":  localAddr.String(),
				"publicAddr": publicAddr.String(),
			},
		}
	})
}

// 连接到对等节点
func connectToPeerWASM() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if wasmClient == nil {
			return map[string]interface{}{
				"success": false,
				"error":   "WASM客户端未初始化",
			}
		}

		if len(args) < 3 {
			return map[string]interface{}{
				"success": false,
				"error":   "缺少参数: peerID, peerIP, peerPort",
			}
		}

		peerID := args[0].String()
		peerIP := args[1].String()
		peerPort := args[2].Int()

		// 创建对等节点地址
		peerAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", peerIP, peerPort))
		if err != nil {
			return map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("解析对等节点地址失败: %v", err),
			}
		}

		// 连接到对等节点
		err = wasmClient.p2pManager.ConnectToPeer(peerID, peerAddr)
		if err != nil {
			return map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("连接失败: %v", err),
			}
		}

		return map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("正在连接到节点 %s", peerID),
		}
	})
}

// 发送消息到对等节点
func sendMessageWASM() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if wasmClient == nil {
			return map[string]interface{}{
				"success": false,
				"error":   "WASM客户端未初始化",
			}
		}

		if len(args) < 2 {
			return map[string]interface{}{
				"success": false,
				"error":   "缺少参数: peerID, message",
			}
		}

		peerID := args[0].String()
		message := args[1].String()

		err := wasmClient.p2pManager.SendDataToPeer(peerID, []byte(message))
		if err != nil {
			return map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("发送消息失败: %v", err),
			}
		}

		return map[string]interface{}{
			"success": true,
			"message": "消息已发送",
		}
	})
}

// 获取连接状态
func getConnectionStatusWASM() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if wasmClient == nil {
			return map[string]interface{}{
				"success": false,
				"error":   "WASM客户端未初始化",
			}
		}

		connections := wasmClient.p2pManager.GetAllConnections()
		result := make(map[string]interface{})

		for peerKey, conn := range connections {
			result[peerKey] = map[string]interface{}{
				"remoteIP":   conn.RemoteIP,
				"remotePort": conn.RemotePort,
				"isActive":   conn.IsActive,
				"lastSeen":   conn.LastSeen.Format(time.RFC3339),
			}
		}

		return map[string]interface{}{
			"success":     true,
			"connections": result,
		}
	})
}

// 检查是否连接到指定节点
func isConnectedToPeerWASM() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if wasmClient == nil {
			return map[string]interface{}{
				"success": false,
				"error":   "WASM客户端未初始化",
			}
		}

		if len(args) < 1 {
			return map[string]interface{}{
				"success": false,
				"error":   "缺少peerID参数",
			}
		}

		peerID := args[0].String()
		isConnected := wasmClient.p2pManager.IsConnectedToPeer(peerID)

		return map[string]interface{}{
			"success":   true,
			"connected": isConnected,
			"peerID":    peerID,
		}
	})
}

// 关闭WASM客户端
func closeWASMClient() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if wasmClient != nil {
			wasmClient.p2pManager.Close()
			wasmClient.isRunning = false
			wasmClient = nil
		}

		return map[string]interface{}{
			"success": true,
			"message": "WASM客户端已关闭",
		}
	})
}

// 日志函数
func logMessage() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) > 0 {
			log.Printf("WASM: %s", args[0].String())
		}
		return nil
	})
}

func main() {
	fmt.Println("GoFileShare WASM模块已加载")

	// 注册JavaScript函数
	js.Global().Set("initWASMClient", initWASMClient())
	js.Global().Set("connectToPeer", connectToPeerWASM())
	js.Global().Set("sendMessage", sendMessageWASM())
	js.Global().Set("getConnectionStatus", getConnectionStatusWASM())
	js.Global().Set("isConnectedToPeer", isConnectedToPeerWASM())
	js.Global().Set("closeWASMClient", closeWASMClient())
	js.Global().Set("logMessage", logMessage())

	// 通知JavaScript WASM模块已准备就绪
	js.Global().Call("postMessage", map[string]interface{}{
		"type":    "wasm_ready",
		"message": "GoFileShare WASM模块已准备就绪",
	})

	// 保持程序运行
	select {}
}
