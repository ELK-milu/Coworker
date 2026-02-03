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
