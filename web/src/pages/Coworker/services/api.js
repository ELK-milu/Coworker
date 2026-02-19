/*
Copyright (C) 2025 QuantumNous
*/

/**
 * Coworker REST API Service
 * 侧边栏功能使用 REST API，聊天功能保持 WebSocket
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

export async function listSessions(userId) {
  return request(`/sessions?user_id=${encodeURIComponent(userId)}`);
}

export async function createSession(userId) {
  return request('/sessions', {
    method: 'POST',
    body: JSON.stringify({ user_id: userId }),
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

export async function listTasks(userId, listId = 'default') {
  return request(`/tasks?user_id=${encodeURIComponent(userId)}&list_id=${encodeURIComponent(listId)}`);
}

export async function createTask(userId, taskData, listId = 'default') {
  return request('/tasks', {
    method: 'POST',
    body: JSON.stringify({
      user_id: userId,
      list_id: listId,
      ...taskData,
    }),
  });
}

export async function updateTask(userId, taskId, updates, listId = 'default') {
  return request(`/tasks/${encodeURIComponent(taskId)}`, {
    method: 'PUT',
    body: JSON.stringify({
      user_id: userId,
      list_id: listId,
      ...updates,
    }),
  });
}

export async function deleteTask(userId, taskId, listId = 'default') {
  return request(`/tasks/${encodeURIComponent(taskId)}?user_id=${encodeURIComponent(userId)}&list_id=${encodeURIComponent(listId)}`, {
    method: 'DELETE',
  });
}

export async function reorderTasks(userId, taskIds, listId = 'default') {
  return request('/tasks/reorder', {
    method: 'PUT',
    body: JSON.stringify({
      user_id: userId,
      list_id: listId,
      task_ids: taskIds,
    }),
  });
}

// ========== 文件管理 API ==========

export async function listFiles(userId, path = '') {
  return request(`/files?user_id=${encodeURIComponent(userId)}&path=${encodeURIComponent(path)}`);
}

export async function createFolder(userId, path) {
  return request('/files/folder', {
    method: 'POST',
    body: JSON.stringify({
      user_id: userId,
      path,
    }),
  });
}

export async function deleteFile(userId, path) {
  return request(`/files?user_id=${encodeURIComponent(userId)}&path=${encodeURIComponent(path)}`, {
    method: 'DELETE',
  });
}

export async function renameFile(userId, path, newName) {
  return request('/files/rename', {
    method: 'PUT',
    body: JSON.stringify({
      user_id: userId,
      path,
      new_name: newName,
    }),
  });
}

// 文件上传
export async function uploadFile(userId, path, file) {
  const formData = new FormData();
  formData.append('file', file);
  formData.append('user_id', userId);
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
export function getDownloadUrl(userId, path) {
  return `${API_BASE}/files/download?user_id=${encodeURIComponent(userId)}&path=${encodeURIComponent(path)}`;
}

// 文件预览 URL（inline + 正确 MIME type）
export function getPreviewUrl(userId, path) {
  return `${API_BASE}/files/preview?user_id=${encodeURIComponent(userId)}&path=${encodeURIComponent(path)}`;
}

// 保存编辑后的文件（覆盖原文件）
export async function saveFileContent(userId, filePath, blob, fileName) {
  const dir = filePath.includes('/') ? filePath.substring(0, filePath.lastIndexOf('/')) : '';
  const file = new File([blob], fileName, { type: blob.type });
  return uploadFile(userId, dir, file);
}

// ========== 配置管理 API ==========

export async function getConfig(userId) {
  return request(`/config?user_id=${encodeURIComponent(userId)}`);
}

export async function saveConfig(userId, content) {
  return request('/config', {
    method: 'PUT',
    body: JSON.stringify({
      user_id: userId,
      content,
    }),
  });
}

// ========== 用户信息 API ==========

export async function getUserInfo(userId) {
  return request(`/userinfo?user_id=${encodeURIComponent(userId)}`);
}

export async function saveUserInfo(userId, userInfo) {
  return request('/userinfo', {
    method: 'PUT',
    body: JSON.stringify({
      user_id: userId,
      user_name: userInfo.userName,
      coworker_name: userInfo.coworkerName,
      assistant_avatar: userInfo.assistantAvatar || '',
      phone: userInfo.phone,
      email: userInfo.email,
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

export async function listMemories(userId) {
  return request(`/memories?user_id=${encodeURIComponent(userId)}`);
}

export async function getMemory(userId, memoryId) {
  return request(`/memories/${encodeURIComponent(memoryId)}?user_id=${encodeURIComponent(userId)}`);
}

export async function createMemory(userId, memoryData) {
  return request('/memories', {
    method: 'POST',
    body: JSON.stringify({
      user_id: userId,
      ...memoryData,
    }),
  });
}

export async function updateMemory(userId, memoryId, updates) {
  return request(`/memories/${encodeURIComponent(memoryId)}`, {
    method: 'PUT',
    body: JSON.stringify({
      user_id: userId,
      ...updates,
    }),
  });
}

export async function deleteMemory(userId, memoryId) {
  return request(`/memories/${encodeURIComponent(memoryId)}?user_id=${encodeURIComponent(userId)}`, {
    method: 'DELETE',
  });
}

export async function searchMemories(userId, query) {
  return request(`/memories/search?user_id=${encodeURIComponent(userId)}&q=${encodeURIComponent(query)}`);
}

// ========== Job 管理 API ==========

export async function listJobs(userId) {
  return request(`/jobs?user_id=${encodeURIComponent(userId)}`);
}

export async function createJob(userId, jobData) {
  return request('/jobs', {
    method: 'POST',
    body: JSON.stringify({
      user_id: userId,
      ...jobData,
    }),
  });
}

export async function updateJob(userId, jobId, updates) {
  return request(`/jobs/${encodeURIComponent(jobId)}`, {
    method: 'PUT',
    body: JSON.stringify({
      user_id: userId,
      ...updates,
    }),
  });
}

export async function deleteJob(userId, jobId) {
  return request(`/jobs/${encodeURIComponent(jobId)}?user_id=${encodeURIComponent(userId)}`, {
    method: 'DELETE',
  });
}

export async function runJob(userId, jobId) {
  return request(`/jobs/${encodeURIComponent(jobId)}/run`, {
    method: 'POST',
    body: JSON.stringify({
      user_id: userId,
    }),
  });
}

export async function reorderJobs(userId, jobIds) {
  return request('/jobs/reorder', {
    method: 'PUT',
    body: JSON.stringify({
      user_id: userId,
      job_ids: jobIds,
    }),
  });
}
