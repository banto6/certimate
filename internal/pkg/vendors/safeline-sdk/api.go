package safelinesdk

func (c *Client) UpdateCertificate(req *UpdateCertificateRequest) (*UpdateCertificateResponse, error) {
	resp := UpdateCertificateResponse{}
	err := c.sendRequestWithResult("/api/open/cert", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
