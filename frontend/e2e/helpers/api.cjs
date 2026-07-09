const API_BASE = 'http://localhost:8080';

async function apiCall(path, options = {}) {
  const response = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  });
  return response;
}

async function apiAuth(path, token, options = {}) {
  return apiCall(path, {
    ...options,
    headers: { Authorization: `Bearer ${token}`, ...options.headers },
  });
}

// Unwrap {success, data, error} envelope from API responses.
function unwrap(body) {
  if (body && typeof body === 'object' && 'success' in body) {
    return body.data;
  }
  return body;
}

async function getToken(username, password = 'testpass123') {
  const res = await apiCall('/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });
  const data = unwrap(await res.json());
  return data.token;
}

async function createOrganization(token, name) {
  // type обязателен с #1046 - иначе POST вернёт 400.
  const res = await apiAuth('/organizations', token, {
    method: 'POST',
    body: JSON.stringify({ name, type: 'Организация' }),
  });
  return unwrap(await res.json());
}

async function createCompany(token, name, organizationId) {
  // type обязателен с #1046 - иначе POST вернёт 400.
  const res = await apiAuth('/companies', token, {
    method: 'POST',
    body: JSON.stringify({ name, type: 'Организация', organization_id: organizationId }),
  });
  return unwrap(await res.json());
}

async function getAttachments(token) {
  const res = await apiAuth('/attachments', token);
  return unwrap(await res.json());
}

async function createUniqueAttachment(token, data) {
  const res = await apiAuth('/attachments', token, {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return unwrap(await res.json());
}

async function submitCompleteApplication(token, data) {
  const res = await apiAuth('/applications/submit-complete-application', token, {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return unwrap(await res.json());
}

async function getApplications(token, params = '') {
  const res = await apiAuth(`/applications${params ? '?' + params : ''}`, token);
  return unwrap(await res.json());
}

async function getUserApplications(token, params = '') {
  const res = await apiAuth(`/applications/user${params ? '?' + params : ''}`, token);
  return unwrap(await res.json());
}

async function createUniqueCar(token, data) {
  const res = await apiAuth('/unique-cars', token, {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return unwrap(await res.json());
}

async function getUniqueCars(token, filterType = 'user') {
  const res = await apiAuth(`/unique-cars?filter_type=${filterType}`, token);
  return unwrap(await res.json());
}

async function createUniqueEmployee(token, data) {
  const res = await apiAuth('/unique-employees', token, {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return unwrap(await res.json());
}

async function getUniqueEmployees(token, filterType = 'user') {
  const res = await apiAuth(`/unique-employees?filter_type=${filterType}`, token);
  return unwrap(await res.json());
}

async function setUserOrganization(token, username, orgId) {
  const res = await apiAuth(`/users/${username}/organization`, token, {
    method: 'PUT',
    body: JSON.stringify({ organization_id: orgId }),
  });
  return unwrap(await res.json());
}

async function setUserCompany(token, username, companyId) {
  const res = await apiAuth(`/users/${username}/company`, token, {
    method: 'PUT',
    body: JSON.stringify({ company_id: companyId }),
  });
  return unwrap(await res.json());
}

async function forwardApplication(token, appId, data) {
  const res = await apiAuth(`/applications/${appId}/forward`, token, {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return unwrap(await res.json());
}

async function approveApplication(token, appId, data) {
  const res = await apiAuth(`/applications/${appId}/approve`, token, {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return unwrap(await res.json());
}

async function takeToWork(token, appId, data) {
  const res = await apiAuth(`/applications/${appId}/take-to-work`, token, {
    method: 'POST',
    body: JSON.stringify(data),
  });
  return unwrap(await res.json());
}

module.exports = {
  apiCall,
  apiAuth,
  unwrap,
  getToken,
  createOrganization,
  createCompany,
  getAttachments,
  createUniqueAttachment,
  submitCompleteApplication,
  getApplications,
  getUserApplications,
  createUniqueCar,
  getUniqueCars,
  createUniqueEmployee,
  getUniqueEmployees,
  setUserOrganization,
  setUserCompany,
  forwardApplication,
  approveApplication,
  takeToWork,
};
