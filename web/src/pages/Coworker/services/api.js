/*
Copyright (C) 2025 QuantumNous
*/

/**
 * Coworker REST API Service
 * 侧边栏功能使用 REST API，聊天功能保持 WebSocket
 * user_id 由后端从 session cookie 中读取，前端无需传递
 */

const API_BASE = '/coworker';

// 通用请求方法
async function request(url, options = {}) {
  const response = await fetch(`${API_BASE}${url}`, {
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
    ...options,
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(error.error || 'Request failed');
  }

  return response.json();
}

// ========== 会话管理 API ==========

export async function listSessions() {
  return request('/sessions');
}

export async function createSession() {
  return request('/sessions', {
    method: 'POST',
    body: JSON.stringify({}),
  });
}

export async function getSession(sessionId) {
  return request(`/sessions/${encodeURIComponent(sessionId)}`);
}

export async function getSessionHistory(sessionId) {
  return request(`/sessions/${encodeURIComponent(sessionId)}/history`);
}

export async function deleteSession(sessionId) {
  return request(`/sessions/${encodeURIComponent(sessionId)}`, {
    method: 'DELETE',
  });
}

// ========== 任务管理 API ==========

export async function listTasks(listId = 'default') {
  return request(`/tasks?list_id=${encodeURIComponent(listId)}`);
}

export async function createTask(taskData, listId = 'default') {
  return request('/tasks', {
    method: 'POST',
    body: JSON.stringify({
      list_id: listId,
      ...taskData,
    }),
  });
}

export async function updateTask(taskId, updates, listId = 'default') {
  return request(`/tasks/${encodeURIComponent(taskId)}`, {
    method: 'PUT',
    body: JSON.stringify({
      list_id: listId,
      ...updates,
    }),
  });
}

export async function deleteTask(taskId, listId = 'default') {
  return request(`/tasks/${encodeURIComponent(taskId)}?list_id=${encodeURIComponent(listId)}`, {
    method: 'DELETE',
  });
}

export async function reorderTasks(taskIds, listId = 'default') {
  return request('/tasks/reorder', {
    method: 'PUT',
    body: JSON.stringify({
      list_id: listId,
      task_ids: taskIds,
    }),
  });
}

// ========== 文件管理 API ==========

export async function listFiles(path = '') {
  return request(`/files?path=${encodeURIComponent(path)}`);
}

export async function createFolder(path) {
  return request('/files/folder', {
    method: 'POST',
    body: JSON.stringify({ path }),
  });
}

export async function deleteFile(path) {
  return request(`/files?path=${encodeURIComponent(path)}`, {
    method: 'DELETE',
  });
}

export async function renameFile(path, newName) {
  return request('/files/rename', {
    method: 'PUT',
    body: JSON.stringify({
      path,
      new_name: newName,
    }),
  });
}

// 文件上传
export async function uploadFile(path, file) {
  const formData = new FormData();
  formData.append('file', file);
  formData.append('path', path);

  const response = await fetch(`${API_BASE}/files/upload`, {
    method: 'POST',
    body: formData,
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(error.error || 'Upload failed');
  }

  return response.json();
}

// 文件下载 URL
export function getDownloadUrl(path) {
  return `${API_BASE}/files/download?path=${encodeURIComponent(path)}`;
}

// 文件预览 URL（inline + 正确 MIME type）
export function getPreviewUrl(path) {
  return `${API_BASE}/files/preview?path=${encodeURIComponent(path)}`;
}

// 保存编辑后的文件（覆盖原文件）
export async function saveFileContent(filePath, blob, fileName) {
  const dir = filePath.includes('/') ? filePath.substring(0, filePath.lastIndexOf('/')) : '';
  const file = new File([blob], fileName, { type: blob.type });
  return uploadFile(dir, file);
}

// 工作空间使用统计
export async function getWorkspaceStats() {
  return request('/files/stats');
}

// ========== 配置管理 API ==========

export async function getConfig() {
  return request('/config');
}

export async function saveConfig(content) {
  return request('/config', {
    method: 'PUT',
    body: JSON.stringify({ content }),
  });
}

// ========== 用户信息 API ==========

export async function getUserInfo() {
  return request('/userinfo');
}

export async function saveUserInfo(userInfo) {
  return request('/userinfo', {
    method: 'PUT',
    body: JSON.stringify({
      user_name: userInfo.userName,
      coworker_name: userInfo.coworkerName,
      assistant_avatar: userInfo.assistantAvatar || '',
      phone: userInfo.phone,
      email: userInfo.email,
      wechat_id: userInfo.wechatId || '',
      api_token_key: userInfo.apiTokenKey || '',
      api_token_name: userInfo.apiTokenName || '',
      selected_model: userInfo.selectedModel || '',
      group: userInfo.group || '',
      temperature: userInfo.temperature,
      top_p: userInfo.topP,
      frequency_penalty: userInfo.frequencyPenalty,
      presence_penalty: userInfo.presencePenalty,
    }),
  });
}

// ========== 记忆管理 API ==========

export async function listMemories() {
  return request('/memories');
}

export async function getMemory(memoryId) {
  return request(`/memories/${encodeURIComponent(memoryId)}`);
}

export async function createMemory(memoryData) {
  return request('/memories', {
    method: 'POST',
    body: JSON.stringify({ ...memoryData }),
  });
}

export async function updateMemory(memoryId, updates) {
  return request(`/memories/${encodeURIComponent(memoryId)}`, {
    method: 'PUT',
    body: JSON.stringify({ ...updates }),
  });
}

export async function deleteMemory(memoryId) {
  return request(`/memories/${encodeURIComponent(memoryId)}`, {
    method: 'DELETE',
  });
}

export async function searchMemories(query) {
  return request(`/memories/search?q=${encodeURIComponent(query)}`);
}

// ========== Job 管理 API ==========

export async function listJobs() {
  return request('/jobs');
}

export async function createJob(jobData) {
  return request('/jobs', {
    method: 'POST',
    body: JSON.stringify({ ...jobData }),
  });
}

export async function updateJob(jobId, updates) {
  return request(`/jobs/${encodeURIComponent(jobId)}`, {
    method: 'PUT',
    body: JSON.stringify({ ...updates }),
  });
}

export async function deleteJob(jobId) {
  return request(`/jobs/${encodeURIComponent(jobId)}`, {
    method: 'DELETE',
  });
}

export async function runJob(jobId) {
  return request(`/jobs/${encodeURIComponent(jobId)}/run`, {
    method: 'POST',
    body: JSON.stringify({}),
  });
}

export async function reorderJobs(jobIds) {
  return request('/jobs/reorder', {
    method: 'PUT',
    body: JSON.stringify({ job_ids: jobIds }),
  });
}

// ========== MCP 配置 API ==========

export async function getUserMCPConfig(itemId) {
  return request(`/store/user/${encodeURIComponent(itemId)}/config`);
}

export async function saveUserMCPConfig(itemId, mcpJson) {
  return request(`/store/user/${encodeURIComponent(itemId)}/config`, {
    method: 'PUT',
    body: JSON.stringify({ mcp_json: mcpJson }),
  });
}

export async function testMCPConnection(mcpJson, expectedName = '', timeout = 15) {
  return request('/mcp/test', {
    method: 'POST',
    body: JSON.stringify({ mcp_json: mcpJson, expected_name: expectedName, timeout }),
  });
}
