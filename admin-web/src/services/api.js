const API_BASE = import.meta.env.VITE_API_BASE || '/api';
const TOKEN_KEY = 'qdl_token';

export function getToken() {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token) {
  if (token) localStorage.setItem(TOKEN_KEY, token);
  else localStorage.removeItem(TOKEN_KEY);
}

export async function api(path, options = {}) {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(getToken() ? { Authorization: `Bearer ${getToken()}` } : {}),
      ...(options.headers || {}),
    },
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok || body.code !== 0) {
    throw new Error(body.message || '请求失败');
  }
  return body;
}

export function listResource(resource, params = {}) {
  const query = new URLSearchParams(params).toString();
  return api(`/${resource}${query ? `?${query}` : ''}`);
}

export function getResource(resource, id) {
  return api(`/${resource}/${id}`);
}

export function createResource(resource, data) {
  return api(`/${resource}`, { method: 'POST', body: JSON.stringify(data) });
}

export function updateResource(resource, id, data) {
  return api(`/${resource}/${id}`, { method: 'PUT', body: JSON.stringify(data) });
}

export function deleteResource(resource, id) {
  return api(`/${resource}/${id}`, { method: 'DELETE' });
}

export function testDomainAccount(id) {
  return api(`/domain-accounts/${id}/test`, { method: 'POST' });
}

export function listProviderDomains(accountId) {
  return api(`/domain-accounts/${accountId}/provider-domains`);
}

export function listDomainRecords(domainId) {
  return api(`/domains/${domainId}/records`);
}

export function listDomainRecordLines(domainId) {
  return api(`/domains/${domainId}/record-lines`);
}

export function refreshDomainExpires(domainId) {
  return api(`/domains/${domainId}/refresh-expires`, { method: 'POST' });
}

export function createDomainRecord(domainId, data) {
  return api(`/domains/${domainId}/records`, { method: 'POST', body: JSON.stringify(data) });
}

export function updateDomainRecord(domainId, recordId, data) {
  return api(`/domains/${domainId}/records/${recordId}`, { method: 'PUT', body: JSON.stringify(data) });
}

export function deleteDomainRecord(domainId, recordId) {
  return api(`/domains/${domainId}/records/${recordId}`, { method: 'DELETE' });
}

export function submitCertificate(id) {
  return api(`/certificates/${id}/submit`, { method: 'POST' });
}

export function revokeCertificate(id) {
  return api(`/certificates/${id}/revoke`, { method: 'POST' });
}

export function getCertificateDetail(id) {
  return api(`/certificates/${id}/detail`);
}

export function listSSLAccountCertificates(id, params = {}) {
  const query = new URLSearchParams(params).toString();
  return api(`/ssl-accounts/${id}/certificates${query ? `?${query}` : ''}`);
}

export function importSSLAccountCertificates(id) {
  return api(`/ssl-accounts/${id}/certificates/import`, { method: 'POST' });
}
