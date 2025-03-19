﻿package k8ssecret

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	xerrors "github.com/pkg/errors"
	k8sCore "k8s.io/api/core/v1"
	k8sMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/usual2970/certimate/internal/pkg/core/deployer"
	"github.com/usual2970/certimate/internal/pkg/utils/certutil"
)

type DeployerConfig struct {
	// kubeconfig 文件内容。
	KubeConfig string `json:"kubeConfig,omitempty"`
	// Kubernetes 命名空间。
	Namespace string `json:"namespace,omitempty"`
	// Kubernetes Secret 名称。
	SecretName string `json:"secretName"`
	// Kubernetes Secret 类型。
	SecretType string `json:"secretType"`
	// Kubernetes Secret 中用于存放证书的 Key。
	SecretDataKeyForCrt string `json:"secretDataKeyForCrt,omitempty"`
	// Kubernetes Secret 中用于存放私钥的 Key。
	SecretDataKeyForKey string `json:"secretDataKeyForKey,omitempty"`
}

type DeployerProvider struct {
	config *DeployerConfig
	logger *slog.Logger
}

var _ deployer.Deployer = (*DeployerProvider)(nil)

func NewDeployer(config *DeployerConfig) (*DeployerProvider, error) {
	if config == nil {
		panic("config is nil")
	}

	return &DeployerProvider{
		logger: slog.Default(),
		config: config,
	}, nil
}

func (d *DeployerProvider) WithLogger(logger *slog.Logger) deployer.Deployer {
	if logger == nil {
		d.logger = slog.Default()
	} else {
		d.logger = logger
	}
	return d
}

func (d *DeployerProvider) Deploy(ctx context.Context, certPem string, privkeyPem string) (*deployer.DeployResult, error) {
	if d.config.Namespace == "" {
		return nil, errors.New("config `namespace` is required")
	}
	if d.config.SecretName == "" {
		return nil, errors.New("config `secretName` is required")
	}
	if d.config.SecretType == "" {
		return nil, errors.New("config `secretType` is required")
	}
	if d.config.SecretDataKeyForCrt == "" {
		return nil, errors.New("config `secretDataKeyForCrt` is required")
	}
	if d.config.SecretDataKeyForKey == "" {
		return nil, errors.New("config `secretDataKeyForKey` is required")
	}

	certX509, err := certutil.ParseCertificateFromPEM(certPem)
	if err != nil {
		return nil, err
	}

	// 连接
	client, err := createK8sClient(d.config.KubeConfig)
	if err != nil {
		return nil, xerrors.Wrap(err, "failed to create k8s client")
	}

	var secretPayload *k8sCore.Secret
	secretAnnotations := map[string]string{
		"certimate/common-name":       certX509.Subject.CommonName,
		"certimate/subject-sn":        certX509.Subject.SerialNumber,
		"certimate/subject-alt-names": strings.Join(certX509.DNSNames, ","),
		"certimate/issuer-sn":         certX509.Issuer.SerialNumber,
		"certimate/issuer-org":        strings.Join(certX509.Issuer.Organization, ","),
	}

	// 获取 Secret 实例，如果不存在则创建
	secretPayload, err = client.CoreV1().Secrets(d.config.Namespace).Get(context.TODO(), d.config.SecretName, k8sMeta.GetOptions{})
	if err != nil {
		secretPayload = &k8sCore.Secret{
			TypeMeta: k8sMeta.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: k8sMeta.ObjectMeta{
				Name:        d.config.SecretName,
				Annotations: secretAnnotations,
			},
			Type: k8sCore.SecretType(d.config.SecretType),
		}
		secretPayload.Data = make(map[string][]byte)
		secretPayload.Data[d.config.SecretDataKeyForCrt] = []byte(certPem)
		secretPayload.Data[d.config.SecretDataKeyForKey] = []byte(privkeyPem)

		secretPayload, err = client.CoreV1().Secrets(d.config.Namespace).Create(context.TODO(), secretPayload, k8sMeta.CreateOptions{})
		d.logger.Debug("k8s operate 'Secrets.Create'", slog.String("namespace", d.config.Namespace), slog.Any("secret", secretPayload))
		if err != nil {
			return nil, xerrors.Wrap(err, "failed to create k8s secret")
		} else {
			return &deployer.DeployResult{}, nil
		}
	}

	// 更新 Secret 实例
	secretPayload.Type = k8sCore.SecretType(d.config.SecretType)
	if secretPayload.ObjectMeta.Annotations == nil {
		secretPayload.ObjectMeta.Annotations = secretAnnotations
	} else {
		for k, v := range secretAnnotations {
			secretPayload.ObjectMeta.Annotations[k] = v
		}
	}
	if secretPayload.Data == nil {
		secretPayload.Data = make(map[string][]byte)
	}
	secretPayload.Data[d.config.SecretDataKeyForCrt] = []byte(certPem)
	secretPayload.Data[d.config.SecretDataKeyForKey] = []byte(privkeyPem)
	secretPayload, err = client.CoreV1().Secrets(d.config.Namespace).Update(context.TODO(), secretPayload, k8sMeta.UpdateOptions{})
	d.logger.Debug("k8s operate 'Secrets.Update'", slog.String("namespace", d.config.Namespace), slog.Any("secret", secretPayload))
	if err != nil {
		return nil, xerrors.Wrap(err, "failed to update k8s secret")
	}

	return &deployer.DeployResult{}, nil
}

func createK8sClient(kubeConfig string) (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error
	if kubeConfig == "" {
		config, err = rest.InClusterConfig()
	} else {
		kubeConfig, err := clientcmd.NewClientConfigFromBytes([]byte(kubeConfig))
		if err != nil {
			return nil, err
		}
		config, err = kubeConfig.ClientConfig()
	}
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return client, nil
}
