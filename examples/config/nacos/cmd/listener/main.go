package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

func main() {
	server := flag.String("server", "127.0.0.1:8858", "Eden HTTP server address")
	dataID := flag.String("data-id", "demo.properties", "Nacos dataId")
	group := flag.String("group", "DEFAULT_GROUP", "Nacos group")
	namespace := flag.String("namespace", "default", "Nacos namespace/tenant")
	timeout := flag.Duration("timeout", 15*time.Second, "maximum wait for the change callback")
	flag.Parse()

	host, rawPort, err := net.SplitHostPort(*server)
	if err != nil {
		log.Fatalf("invalid -server %q: %v", *server, err)
	}
	port, err := strconv.ParseUint(rawPort, 10, 64)
	if err != nil {
		log.Fatalf("invalid server port %q: %v", rawPort, err)
	}

	cacheRoot := filepath.Join(os.TempDir(), "eden-nacos-config-example")
	client, err := clients.NewConfigClient(vo.NacosClientParam{
		ServerConfigs: []constant.ServerConfig{*constant.NewServerConfig(
			host,
			port,
			constant.WithScheme("http"),
			constant.WithContextPath("/nacos"),
		)},
		ClientConfig: &constant.ClientConfig{
			NamespaceId:         *namespace,
			TimeoutMs:           5000,
			NotLoadCacheAtStart: true,
			CacheDir:            filepath.Join(cacheRoot, "cache"),
			LogDir:              filepath.Join(cacheRoot, "log"),
			LogLevel:            "warn",
		},
	})
	if err != nil {
		log.Fatalf("create Nacos config client: %v", err)
	}

	stamp := time.Now().UnixNano()
	initial := fmt.Sprintf("demo.version=initial-%d\nfeature.enabled=false\n", stamp)
	updated := fmt.Sprintf("demo.version=updated-%d\nfeature.enabled=true\n", stamp)
	identity := vo.ConfigParam{DataId: *dataID, Group: *group}

	published, err := client.PublishConfig(vo.ConfigParam{DataId: *dataID, Group: *group, Content: initial})
	if err != nil || !published {
		log.Fatalf("publish initial config: published=%v err=%v", published, err)
	}
	loaded, err := client.GetConfig(identity)
	if err != nil || loaded != initial {
		log.Fatalf("get initial config: content=%q err=%v", loaded, err)
	}
	fmt.Printf("initial config loaded: %s/%s/%s\n", *namespace, *group, *dataID)

	changed := make(chan string, 4)
	err = client.ListenConfig(vo.ConfigParam{
		DataId: *dataID,
		Group:  *group,
		OnChange: func(callbackNamespace, callbackGroup, callbackDataID, content string) {
			fmt.Printf("change callback: %s/%s/%s => %q\n", callbackNamespace, callbackGroup, callbackDataID, content)
			changed <- content
		},
	})
	if err != nil {
		log.Fatalf("listen config: %v", err)
	}
	defer client.CancelListenConfig(identity)

	time.Sleep(time.Second)
	published, err = client.PublishConfig(vo.ConfigParam{DataId: *dataID, Group: *group, Content: updated})
	if err != nil || !published {
		log.Fatalf("publish updated config: published=%v err=%v", published, err)
	}

	timer := time.NewTimer(*timeout)
	defer timer.Stop()
	for {
		select {
		case content := <-changed:
			if content != updated {
				continue
			}
			loaded, err = client.GetConfig(identity)
			if err != nil || loaded != updated {
				log.Fatalf("reload updated config: content=%q err=%v", loaded, err)
			}
			if deleted, deleteErr := client.DeleteConfig(identity); deleteErr != nil || !deleted {
				log.Fatalf("delete demo config: deleted=%v err=%v", deleted, deleteErr)
			}
			fmt.Println("Nacos Config compatibility demo completed successfully")
			return
		case <-timer.C:
			log.Fatalf("timed out after %s waiting for changed config", *timeout)
		}
	}
}
