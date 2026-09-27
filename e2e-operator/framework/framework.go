package framework

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	snapshot "github.com/kubernetes-csi/external-snapshotter/client/v8/clientset/versioned"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Framework struct {
	KubeClient       kubernetes.Interface
	DynamicClient    dynamic.Interface
	SnapClient       snapshot.Interface
	Config           *rest.Config
	CephCSINamespace string
	RookNamespace    string
	DefaultTimeout   time.Duration
}

var Instance *Framework

func New() (*Framework, error) {
	kubeconfig := Opts.Kubeconfig
	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}
	if kubeconfig == "" {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	kc, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	sc, err := snapshot.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Framework{
		KubeClient:       kc,
		DynamicClient:    dc,
		SnapClient:       sc,
		Config:           cfg,
		CephCSINamespace: Opts.CephCSINamespace,
		RookNamespace:    Opts.RookNamespace,
		DefaultTimeout:   time.Duration(Opts.DefaultTimeout) * time.Second,
	}, nil
}
