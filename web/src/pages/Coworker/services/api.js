/*
Copyright (C) 2025 QuantumNous
*/

/**
 * Coworker REST API Service
 * 使用项目标准的 API axios 实例，鉴权头由拦截器统一处理
 */

import { API } from '../../../helpers/api';

const API_BASE = '/coworker';

// 通用请求方法
async function request(method, url, data = null, config = {}) {
  const fullUrl = `${API_BASE}${url}`;
  let response;
  switch (method) {
    case 'GET':
      response = await API.get(fullUrl, config);
      break;
    case 'POST':
      response = await API.post(fullUrl, data, config);
      break;
    case 'PUT':
      response = await API.put(fullUrl, data, config);
      break;
    case 'DELETE':
      response = await API.delete(fullUrl, config);
      break;
    default:
      throw new Error(`Unsupported method: ${method}`);
  }
  return response.data;
}

// ========== 会话管理 API ==========

export async function listSessions() {
  return request('GET', '/sessions');
}

export async function createSession() {
  return request('POST', '/sessions', {});
}

export async function getSession(sessionId) {
  return request('GET', `/sessions/${encodeURIComponent(sessionId)}`);
}

export async function getSessionHistory(sessionId) {
  return request('GET', `/sessions/${encodeURIComponent(sessionId)}/history`);
}

export async function deleteSession(sessionId) {
  return request('DELETE', `/sessions/${encodeURIComponent(sessionId)}`);
}

// ========== 任务管理 API ==========

export async function listTasks(listId = 'default') {
  return request('GET', `/tasks?list_id=${encodeURIComponent(listId)}`);
}

export async function createTask(taskData, listId = 'default') {
  return request('POST', '/tasks', {
    list_id: listId,
    ...taskData,
  });
}

export async function updateTask(taskId, updates, listId = 'default') {
  return request('PUT', `/tasks/${encodeURIComponent(taskId)}`, {
    list_id: listId,
    ...updates,
  });
}

export async function deleteTask(taskId, listId = 'default') {
  return request('DELETE', `/tasks/${encodeURIComponent(taskId)}?list_id=${encodeURIComponent(listId)}`);
}

export async function reorderTasks(taskIds, listId = 'default') {
  return request('PUT', '/tasks/reorder', {
    list_id: listId,
    task_ids: taskIds,
  });
}

// ========== 文件管理 API ==========

export async function listFiles(path = '') {
  return request('GET', `/files?path=${encodeURIComponent(path)}`);
}

export async function createFolder(path) {
  return request('POST', '/files/folder', { path });
}

export async function deleteFile(path) {
  return request('DELETE', `/files?path=${encodeURIComponent(path)}`);
}

export async function renameFile(path, newName) {
  return request('PUT', '/files/rename', {
    path,
    new_name: newName,
  });
}

// 文件上传
export async function uploadFile(path, file) {
  const formData = new FormData();
  formData.append('file', file);
  formData.append('path', path);

  const response = await API.post(`${API_BASE}/files/upload`, formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  });
  return response.data;
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
  return request('GET', '/files/stats');
}

// ========== 配置管理 API ==========

export async function getConfig() {
  return request('GET', '/config');
}

export async function saveConfig(content) {
  return request('PUT', '/config', { content });
}

// ========== 用户信息 API ==========

export async function getUserInfo() {
  return request('GET', '/userinfo');
}

export async function saveUserInfo(userInfo) {
  return request('PUT', '/userinfo', {
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
  });
}

// ========== 记忆管理 API ==========

export async function listMemories() {
  return request('GET', '/memories');
}

export async function getMemory(memoryId) {
  return request('GET', `/memories/${encodeURIComponent(memoryId)}`);
}

export async function createMemory(memoryData) {
  return request('POST', '/memories', { ...memoryData });
}

export async function updateMemory(memoryId, updates) {
  return request('PUT', `/memories/${encodeURIComponent(memoryId)}`, { ...updates });
}

export async function deleteMemory(memoryId) {
  return request('DELETE', `/memories/${encodeURIComponent(memoryId)}`);
}

export async function searchMemories(query) {
  return request('GET', `/memories/search?q=${encodeURIComponent(query)}`);
}

// ========== Job 管理 API ==========

export async function listJobs() {
  return request('GET', '/jobs');
}

export async function createJob(jobData) {
  return request('POST', '/jobs', { ...jobData });
}

export async function updateJob(jobId, updates) {
  return request('PUT', `/jobs/${encodeURIComponent(jobId)}`, { ...updates });
}

export async function deleteJob(jobId) {
  return request('DELETE', `/jobs/${encodeURIComponent(jobId)}`);
}

export async function runJob(jobId) {
  return request('POST', `/jobs/${encodeURIComponent(jobId)}/run`, {});
}

export async function reorderJobs(jobIds) {
  return request('PUT', '/jobs/reorder', { job_ids: jobIds });
}

// ========== MCP 配置 API ==========

export async function getUserMCPConfig(itemId) {
  return request('GET', `/store/user/${encodeURIComponent(itemId)}/config`);
}

export async function saveUserMCPConfig(itemId, mcpJson) {
  return request('PUT', `/store/user/${encodeURIComponent(itemId)}/config`, { mcp_json: mcpJson });
}

export async function testMCPConnection(mcpJson, expectedName = '', timeout = 15) {
  return request('POST', '/mcp/test', { mcp_json: mcpJson, expected_name: expectedName, timeout });
}

// ========== 技能商店 API ==========

export async function getStoreItems() {
  return request('GET', '/store/items');
}

export async function getStoreUserInstalled() {
  return request('GET', '/store/user');
}

export async function installStoreItem(itemId) {
  return request('POST', `/store/user/install/${encodeURIComponent(itemId)}`, {});
}

export async function uninstallStoreItem(itemId) {
  return request('DELETE', `/store/user/uninstall/${encodeURIComponent(itemId)}`);
}
