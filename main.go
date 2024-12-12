package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	// WindowsシステムAPI を利用するための公式ライブラリ
	"golang.org/x/sys/windows"
)

// WinAPIの関数をGo言語から呼び出すためにDLL（Dynamic Link Library）から関数をロード
var (
	// WindowsのGUI（ウィンドウやメッセージ）関連の関数を提供するuser32.dllをロード
	user32 = syscall.NewLazyDLL("user32.dll")
	// 基本的なWindows API（メモリ管理やプロセス操作など）を提供するkernel32.dllをロード
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	// user32.dllからRegisterClassExW関数をロード
	// ウィンドウクラスを登録する関数
	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	// user32.dllからCreateWindowExW関数をロード
	// 新しいウィンドウを作成する関数
	procCreateWindowExW = user32.NewProc("CreateWindowExW")
	// user32.dllからDefWindowProcW関数をロード
	// デフォルトのウィンドウメッセージ処理を提供する関数
	procDefWindowProcW = user32.NewProc("DefWindowProcW")
	// user32.dllからGetMessageW関数をロード
	// Windowsメッセージを取得する関数
	procGetMessageW = user32.NewProc("GetMessageW")
	// user32.dllからTranslateMessage関数をロード
	// キーボード入力のメッセージを翻訳（処理）する関数
	procTranslateMessage = user32.NewProc("TranslateMessage")
	// user32.dllからDispatchMessageW関数をロード
	// 取得したメッセージを適切なウィンドウプロシージャに送信する関数
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	// Windowsでデバイス情報を操作するAPI群を提供
	setupapi = syscall.NewLazyDLL("setupapi.dll")
	// 特定のデバイスクラスのリストを取得
	procSetupDiGetClassDevsW = setupapi.NewProc("SetupDiGetClassDevsW")
	// デバイスリストを1つずつ列挙
	procSetupDiEnumDeviceInfo = setupapi.NewProc("SetupDiEnumDeviceInfo")
	// 使用済みのデバイスリストを解放
	procSetupDiDestroyDeviceInfoList = setupapi.NewProc("SetupDiDestroyDeviceInfoList")
	// デバイスの製造元やシリアル番号などのプロパティを取得
	procSetupDiGetDeviceRegistryPropertyW = setupapi.NewProc("SetupDiGetDeviceRegistryPropertyW")
)

// WinAPIで使用されるデバイス関連の定数
const (
	// デバイスの状態が変化したとき（接続、切断など）に送信されるメッセージ
	WM_DEVICECHANGE = 0x0219
	// 新しいデバイスが接続されたことを示すイベント
	DBT_DEVICEARRIVAL = 0x8000
	// デバイスが安全に取り外されたことを示すイベント
	DBT_DEVICEREMOVECOMPLETE = 0x8004
	// デバイスの種類を示す値
	// デバイスインターフェースを表す
	DBT_DEVTYP_DEVICEINTERFACE = 0x00000005
	// 現在接続されているデバイスのみを対象にするフラグ
	DIGCF_PRESENT = 0x02
	// デバイスインターフェイス情報を取得するフラグ
	DIGCF_DEVICEINTERFACE = 0x10
	// デバイスの製造元情報（Manufacturer）を取得するプロパティ
	SPDRP_MFG = 0x0000000B
	// ユーザーに表示されるフレンドリ名（Friendly Name）を取得するプロパティ
	SPDRP_FRIENDLYNAME = 0x0000000C
	// デバイスのハードウェアIDを取得するプロパティ
	SPDRP_HARDWAREID = 0x00000001
)

// ウィンドウクラスを定義するための構造体
type Wndclassex struct {
	// 構造体のサイズ
	CbSize uint32
	// ウィンドウクラスのスタイル（例: 描画の仕方）
	Style uint32
	//  メッセージ処理関数（ウィンドウの振る舞いを決定）
	LpfnWndProc uintptr
	// クラスに追加するメモリのサイズ
	CbClsExtra int32
	// ウィンドウごとに追加するメモリのサイズ
	CbWndExtra int32
	// ウィンドウを属するプロセス（モジュール）のハンドル
	HInstance syscall.Handle
	// ウィンドウのアイコン
	HIcon syscall.Handle
	// ウィンドウで使用するカーソル
	HCursor syscall.Handle
	// 背景ブラシ（背景色）
	HbrBackground syscall.Handle
	// メニューバーの名前（省略可能）
	LpszMenuName *uint16
	// ウィンドウクラスの名前
	LpszClassName *uint16
	// 小さいアイコン（タスクバーなどで使用）
	HIconSm syscall.Handle
}

// Windowsのメッセージ情報を格納するための構造体
type Msg struct {
	// メッセージの送信先ウィンドウハンドル
	HWnd syscall.Handle
	// メッセージの種類
	Message uint32
	// メッセージに付随する追加情報
	WParam uintptr
	LParam uintptr
	// メッセージが発生した時刻
	Time uint32
	// マウスイベント時の座標
	Pt struct {
		X int32
		Y int32
	}
}

type DeviceInfo struct {
	// デバイスの製造元を表す情報
	Manufacturer string
	// USBデバイスに固有の情報
	SerialNumber string
}

// DEV_BROADCAST_HDR構造体
type DevBroadcastHdr struct {
	Size       uint32
	DeviceType uint32
	Reserved   uint32
}

// DEV_BROADCAST_DEVICEINTERFACE構造体
type DevBroadcastDeviceInterface struct {
	Size       uint32
	DeviceType uint32
	Reserved   uint32
	ClassGuid  windows.GUID
	Name       [1]uint16 // 可変長文字列
}

func main() {
	// 現在実行中のプロセス（自分自身のモジュール）のハンドルを取得
	hInstance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)

	// ウィンドウクラス名
	// ウィンドウクラス名=ウィンドウクラスを識別するための一意のラベル
	// ウィンドウクラス=ウィンドウの振る舞いやスタイルを定義するテンプレート
	className, _ := windows.UTF16PtrFromString("USBMonitorClass")

	// 仮想的なウィンドウクラスのテンプレートを定義
	wndClass := Wndclassex{
		CbSize:        uint32(unsafe.Sizeof(Wndclassex{})),
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     syscall.Handle(hInstance),
		LpszClassName: className,
	}

	// Windowsシステム（OSのカーネル内）にウィンドウクラスを登録
	_, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wndClass)))
	if err != nil && err.Error() != "The operation completed successfully." {
		fmt.Println("Failed to register window class:", err)
		return
	}

	// テンプレートを基に、仮想的なウィンドウを作成
	title, _ := windows.UTF16PtrFromString("USB Monitor")
	hWnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(wndClass.LpszClassName)),
		uintptr(unsafe.Pointer(title)),
		0,
		0, 0, 0, 0,
		0, 0,
		// 作成するウィンドウを関連付けるプロセス（モジュール）のハンドル
		uintptr(hInstance), 0,
	)
	if hWnd == 0 {
		fmt.Println("Failed to create window:", err)
		return
	}

	// Windowsの右下に通知を表示
	var msg Msg
	for {
		// システムからメッセージを取得
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 {
			break
		}
		// キーボード入力に関連するメッセージの補助処理
		// キー入力を処理するときに、文字そのものを扱えるようにする
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		// メッセージをLpfnWndProcで処理
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func wndProc(hWnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_DEVICECHANGE:
		if wParam == DBT_DEVICEARRIVAL {
			deviceInfo := getDeviceInfo()
			hostName := getHostName()
			logDeviceInfo(deviceInfo, hostName)
		}
	}
	// 自分で処理しないメッセージ（例: ウィンドウの最小化、移動、閉じる操作など）をWindowsに処理を依頼
	ret, _, _ := procDefWindowProcW.Call(uintptr(hWnd), uintptr(msg), wParam, lParam)
	return ret
}

func getDeviceInfo() DeviceInfo {
	// USBデバイス全体を対象にしたデバイスクラスGUID
	var usbGuid = windows.GUID{
		Data1: 0x36FC9E60,
		Data2: 0xC465,
		Data3: 0x11CF,
		Data4: [8]byte{0x80, 0x56, 0x44, 0x45, 0x53, 0x54, 0x00, 0x00},
	}

	// 現在接続されているUSBデバイスのリストのハンドルを取得
	hDevInfo, _, _ := procSetupDiGetClassDevsW.Call(
		uintptr(unsafe.Pointer(&usbGuid)),
		0,
		0,
		DIGCF_PRESENT,
	)
	// ハンドルを使用後に解放するようスケジュール
	defer procSetupDiDestroyDeviceInfoList.Call(hDevInfo)

	// デバイス情報（GUID、インスタンス情報など）を格納するための構造体を作成
	var deviceInfoData struct {
		CbSize    uint32
		ClassGuid windows.GUID
		DevInst   uint32
		Reserved  uintptr
	}
	// 初期化
	deviceInfoData.CbSize = uint32(unsafe.Sizeof(deviceInfoData))

	// デバイスリストのハンドル内のデバイス情報を1つ取得
	if ret, _, _ := procSetupDiEnumDeviceInfo.Call(hDevInfo, 0, uintptr(unsafe.Pointer(&deviceInfoData))); ret == 0 {
		fmt.Println("Failed to enumerate device.")
		return DeviceInfo{}
	}

	var buffer [256]uint16
	propertyRegDataType := uint32(0)
	requiredSize := uint32(0)

	// 製造元の取得
	procSetupDiGetDeviceRegistryPropertyW.Call(
		hDevInfo,
		uintptr(unsafe.Pointer(&deviceInfoData)),
		SPDRP_MFG,
		uintptr(unsafe.Pointer(&propertyRegDataType)),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)*2),
		uintptr(unsafe.Pointer(&requiredSize)),
	)
	manufacturer := windows.UTF16ToString(buffer[:])

	// シリアル番号(Hardware ID)の取得
	procSetupDiGetDeviceRegistryPropertyW.Call(
		hDevInfo,
		uintptr(unsafe.Pointer(&deviceInfoData)),
		SPDRP_HARDWAREID,
		uintptr(unsafe.Pointer(&propertyRegDataType)),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)*2),
		uintptr(unsafe.Pointer(&requiredSize)),
	)
	serialNumber := windows.UTF16ToString(buffer[:])

	return DeviceInfo{
		Manufacturer: manufacturer,
		SerialNumber: serialNumber,
	}
}

func logDeviceInfo(deviceInfo DeviceInfo, hostName string) {
	fmt.Print("Connected: ")
	fmt.Printf("Host=%s, ", hostName)
	fmt.Printf("Device Manufacturer=%s, ", deviceInfo.Manufacturer)
	fmt.Printf("Serial Number=%s\n", deviceInfo.SerialNumber)
}

func getHostName() string {
	hostName, err := os.Hostname()
	if err != nil {
		return "Unknown Host"
	}
	return hostName
}
