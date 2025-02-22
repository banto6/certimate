package cacheflysdk

import (
	"net/http"
)

func (c *Client) CreateCertificate(req *CreateCertificateRequest) (*CreateCertificateResponse, error) {
	resp := CreateCertificateResponse{}
	err := c.sendRequestWithResult(http.MethodPost, "/certificates", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
