package policy

func policyToResponse(policy *AccessPolicy) *Response {
	if policy == nil {
		return nil
	}
	return &Response{ID: policy.ID, Kind: policy.Kind, PrincipalType: policy.PrincipalType,
		Principal: policy.Principal, Reason: policy.Reason, Enabled: policy.Enabled,
		ExpiresAt: policy.ExpiresAt, CreatedAt: policy.CreatedAt, UpdatedAt: policy.UpdatedAt}
}

func policiesToResponse(policies []*AccessPolicy) []*Response {
	if policies == nil {
		return nil
	}
	responses := make([]*Response, len(policies))
	for i, policy := range policies {
		responses[i] = policyToResponse(policy)
	}
	return responses
}
