﻿package volcenginedcdn

import (
	"context"
	"strings"

	xerrors "github.com/pkg/errors"
	veDcdn "github.com/volcengine/volcengine-go-sdk/service/dcdn"
	ve "github.com/volcengine/volcengine-go-sdk/volcengine"
	veSession "github.com/volcengine/volcengine-go-sdk/volcengine/session"

	"github.com/usual2970/certimate/internal/pkg/core/deployer"
	"github.com/usual2970/certimate/internal/pkg/core/logger"
	"github.com/usual2970/certimate/internal/pkg/core/uploader"
	uploadersp "github.com/usual2970/certimate/internal/pkg/core/uploader/providers/volcengine-certcenter"
)

type DeployerConfig struct {
	// 火山引擎 AccessKeyId。
	AccessKeyId string `json:"accessKeyId"`
	// 火山引擎 AccessKeySecret。
	AccessKeySecret string `json:"accessKeySecret"`
	// 火山引擎地域。
	Region string `json:"region"`
	// 加速域名（支持泛域名）。
	Domain string `json:"domain"`
}

type DeployerProvider struct {
	config      *DeployerConfig
	logger      logger.Logger
	sdkClient   *veDcdn.DCDN
	sslUploader uploader.Uploader
}

var _ deployer.Deployer = (*DeployerProvider)(nil)

func NewDeployer(config *DeployerConfig) (*DeployerProvider, error) {
	if config == nil {
		panic("config is nil")
	}

	client, err := createSdkClient(config.AccessKeyId, config.AccessKeySecret, config.Region)
	if err != nil {
		return nil, xerrors.Wrap(err, "failed to create sdk client")
	}

	uploader, err := uploadersp.NewUploader(&uploadersp.UploaderConfig{
		AccessKeyId:     config.AccessKeyId,
		AccessKeySecret: config.AccessKeySecret,
		Region:          config.Region,
	})
	if err != nil {
		return nil, xerrors.Wrap(err, "failed to create ssl uploader")
	}

	return &DeployerProvider{
		config:      config,
		logger:      logger.NewNilLogger(),
		sdkClient:   client,
		sslUploader: uploader,
	}, nil
}

func (d *DeployerProvider) WithLogger(logger logger.Logger) *DeployerProvider {
	d.logger = logger
	return d
}

func (d *DeployerProvider) Deploy(ctx context.Context, certPem string, privkeyPem string) (*deployer.DeployResult, error) {
	// 上传证书到证书中心
	upres, err := d.sslUploader.Upload(ctx, certPem, privkeyPem)
	if err != nil {
		return nil, xerrors.Wrap(err, "failed to upload certificate file")
	}

	d.logger.Logt("certificate file uploaded", upres)

	// "*.example.com" → ".example.com"，适配火山引擎 DCDN 要求的泛域名格式
	domain := strings.TrimPrefix(d.config.Domain, "*")

	// 绑定证书
	// REF: https://www.volcengine.com/docs/6559/1250189
	createCertBindReq := &veDcdn.CreateCertBindInput{
		CertSource:  ve.String("volc"),
		CertId:      ve.String(upres.CertId),
		DomainNames: ve.StringSlice([]string{domain}),
	}
	createCertBindResp, err := d.sdkClient.CreateCertBind(createCertBindReq)
	if err != nil {
		return nil, xerrors.Wrap(err, "failed to execute sdk request 'dcdn.CreateCertBind'")
	} else {
		d.logger.Logt("已绑定证书", createCertBindResp)
	}

	return &deployer.DeployResult{}, nil
}

func createSdkClient(accessKeyId, accessKeySecret, region string) (*veDcdn.DCDN, error) {
	if region == "" {
		region = "cn-beijing" // DCDN 服务默认区域：北京
	}

	config := ve.NewConfig().WithRegion(region).WithAkSk(accessKeyId, accessKeySecret)

	session, err := veSession.NewSession(config)
	if err != nil {
		return nil, err
	}

	client := veDcdn.New(session)
	return client, nil
}
