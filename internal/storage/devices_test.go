package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDeviceGroupsAndDevicesPersistAndAssociateNetworkNode(t *testing.T) {
	ctx:=context.Background()
	path:=filepath.Join(t.TempDir(),"data","app.db")
	db,err:=Open(path);if err!=nil{t.Fatal(err)};defer db.Close()
	store,err:=NewDeviceStore(ctx,db);if err!=nil{t.Fatal(err)}
	group,err:=store.CreateDeviceGroup(ctx,"家庭");if err!=nil{t.Fatal(err)}
	device,err:=store.CreateDevice(ctx,DeviceInput{
		Name:"客厅电视",GroupID:group.ID,DeviceType:"tv",Manufacturer:"小米",Model:"电视",
		IPAddress:"192.168.1.20",MACAddress:"AA:BB:CC:DD:EE:FF",NetworkKey:"network:home",PeerID:"peer-1",
	});if err!=nil{t.Fatal(err)}
	if device.GroupName!="家庭"||device.MACAddress!="aa:bb:cc:dd:ee:ff"||device.PeerID!="peer-1"{t.Fatalf("设备字段错误: %+v",device)}
	if _,err:=store.CreateDevice(ctx,DeviceInput{Name:"重复节点",DeviceType:"other",NetworkKey:"network:home",PeerID:"peer-1"});!errors.Is(err,ErrDeviceNodeConflict){t.Fatalf("允许同一节点关联多个设备: %v",err)}
	groups,err:=store.ListDeviceGroups(ctx);if err!=nil||len(groups)!=1||groups[0].DeviceCount!=1{t.Fatalf("分组统计错误: %+v %v",groups,err)}
	updated,err:=store.UpdateDevice(ctx,device.ID,DeviceInput{Name:"客厅电视盒子",DeviceType:"tv",IPAddress:"192.168.1.21"});if err!=nil{t.Fatal(err)}
	if updated.Name!="客厅电视盒子"||updated.GroupID!=""||updated.NetworkKey!=""{t.Fatalf("设备更新未正确覆盖关联字段: %+v",updated)}
	renamed,err:=store.UpdateDeviceGroup(ctx,group.ID,"家庭娱乐");if err!=nil||renamed.Name!="家庭娱乐"{t.Fatalf("分组重命名失败: %+v %v",renamed,err)}
	if err:=store.DeleteDeviceGroup(ctx,group.ID);err!=nil{t.Fatal(err)}
	if _,err:=store.Device(ctx,device.ID);err!=nil{t.Fatal("删除分组不应删除设备:",err)}
	if err:=store.DeleteDevice(ctx,device.ID);err!=nil{t.Fatal(err)}
	if _,err:=store.Device(ctx,device.ID);!errors.Is(err,ErrDeviceNotFound){t.Fatalf("设备删除失败: %v",err)}
}

func TestDeviceValidationAndOpenWrtCredentialSealing(t *testing.T) {
	ctx:=context.Background();dir:=t.TempDir()
	db,err:=Open(filepath.Join(dir,"app.db"));if err!=nil{t.Fatal(err)};defer db.Close()
	store,err:=NewDeviceStore(ctx,db);if err!=nil{t.Fatal(err)}
	for _,input:=range []DeviceInput{
		{Name:"",DeviceType:"other"},
		{Name:"bad ip",DeviceType:"other",IPAddress:"not-an-ip"},
		{Name:"bad mac",DeviceType:"other",MACAddress:"invalid"},
		{Name:"invalid node link",DeviceType:"other",NetworkKey:"network:home"},
		{Name:"bad OpenWrt",DeviceType:"other",IsOpenWrt:true,IPAddress:"192.168.1.1"},
	} { if _,err:=store.CreateDevice(ctx,input);!errors.Is(err,ErrInvalidDevice){t.Fatalf("非法设备通过校验: %+v err=%v",input,err)} }
	normal,err:=store.CreateDevice(ctx,DeviceInput{Name:"普通路由器",DeviceType:"router",IPAddress:"192.168.1.1"});if err!=nil{t.Fatal(err)}
	if _,err:=store.DeviceOpenWrtConfig(ctx,normal.ID);!errors.Is(err,ErrDeviceNotOpenWrt){t.Fatalf("普通设备可读取 OpenWrt 配置: %v",err)}
	router,err:=store.CreateDevice(ctx,DeviceInput{Name:"OpenWrt",DeviceType:"router",IPAddress:"192.168.1.2",IsOpenWrt:true});if err!=nil{t.Fatal(err)}
	config,err:=store.SaveDeviceOpenWrtConfig(ctx,router.ID,80,false,"easy-device","secret-value");if err!=nil{t.Fatal(err)}
	if !config.Configured||config.Username!="easy-device"{t.Fatalf("OpenWrt 配置未保存: %+v",config)}
	secret,err:=store.DeviceOpenWrtSecret(ctx,router.ID);if err!=nil{t.Fatal(err)}
	if secret.Password!="secret-value"||secret.URL!="http://192.168.1.2:80/ubus"{t.Fatalf("OpenWrt 凭据解封错误: %+v",secret)}
	devices,err:=store.ListDevices(ctx);if err!=nil{t.Fatal(err)}
	for _,d:=range devices { if d.ID==router.ID && !d.OpenWrtConfigured {t.Fatal("列表未显示已配置状态")} }
	if _,err:=store.SaveDeviceOpenWrtConfig(ctx,router.ID,8080,false,"easy-device","");!errors.Is(err,ErrInvalidOpenWrtConfig){t.Fatal("修改配置却未重新提供密码:",err)}
	if _,err:=store.SaveDeviceOpenWrtConfig(ctx,router.ID,80,false,"easy-device","");err!=nil{t.Fatal("相同配置应保留已保存凭据:",err)}
	if _,err:=store.UpdateDevice(ctx,router.ID,DeviceInput{Name:"普通路由器",DeviceType:"router",IPAddress:"192.168.1.2"});err!=nil{t.Fatal(err)}
	if _,err:=store.DeviceOpenWrtConfig(ctx,router.ID);!errors.Is(err,ErrDeviceNotOpenWrt){t.Fatalf("取消 OpenWrt 标识后仍可读取配置: %v",err)}
	info,err:=os.Stat(filepath.Join(dir,".device-openwrt-sealing-key"));if err!=nil||info.Mode().Perm()!=0600{t.Fatalf("密钥文件权限错误: %v %v",info,err)}
}
