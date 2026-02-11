import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

describe('ApiService', () => {
  let apiService;
  let fetchSpy;

  beforeEach(async () => {
    vi.resetModules();
    localStorage.clear();

    // Suppress console.log/debug from api.js initialization
    vi.spyOn(console, 'log').mockImplementation(() => {});
    vi.spyOn(console, 'debug').mockImplementation(() => {});
    vi.spyOn(console, 'warn').mockImplementation(() => {});

    // Mock fetch globally
    fetchSpy = vi.fn();
    vi.stubGlobal('fetch', fetchSpy);

    const mod = await import('../../services/api.js');
    apiService = mod.default;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  function mockFetchResponse(data, status = 200) {
    fetchSpy.mockResolvedValueOnce({
      ok: status >= 200 && status < 300,
      status,
      json: () => Promise.resolve(data),
    });
  }

  it('should attach Authorization header when token is in localStorage', async () => {
    localStorage.setItem('token', 'my-jwt');
    mockFetchResponse({ id: '1', username: 'test' });

    await apiService.getMe();

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    const [, options] = fetchSpy.mock.calls[0];
    expect(options.headers['Authorization']).toBe('Bearer my-jwt');
  });

  it('should NOT attach Authorization header without token', async () => {
    mockFetchResponse({ id: '1' });

    await apiService.getMe();

    const [, options] = fetchSpy.mock.calls[0];
    expect(options.headers['Authorization']).toBeUndefined();
  });

  it('should throw on non-OK response', async () => {
    localStorage.setItem('token', 'jwt');
    mockFetchResponse({ error: 'not found' }, 404);

    await expect(apiService.getCall('bad-id')).rejects.toThrow('not found');
  });

  it('should handle 401 by clearing auth and dispatching event', async () => {
    localStorage.setItem('token', 'expired-jwt');
    localStorage.setItem('user', JSON.stringify({ id: '1' }));

    const dispatchSpy = vi.spyOn(window, 'dispatchEvent');
    mockFetchResponse({ error: 'unauthorized' }, 401);

    const result = await apiService.getMe();

    // Should clear localStorage
    expect(localStorage.getItem('token')).toBeNull();
    expect(localStorage.getItem('user')).toBeNull();

    // Should dispatch auth:expired event
    expect(dispatchSpy).toHaveBeenCalled();
    const event = dispatchSpy.mock.calls[0][0];
    expect(event.type).toBe('auth:expired');

    // Should return empty object (not throw)
    expect(result).toEqual({});
  });

  // ---- Auth endpoints ----

  it('register() should POST to /auth/register', async () => {
    mockFetchResponse({ token: 'new-jwt', user: { id: '1' } });

    await apiService.register('alice', 'alice@test.com', 'pass123');

    const [url, opts] = fetchSpy.mock.calls[0];
    expect(url).toContain('/auth/register');
    expect(opts.method).toBe('POST');
    const body = JSON.parse(opts.body);
    expect(body.username).toBe('alice');
    expect(body.email).toBe('alice@test.com');
  });

  it('login() should POST to /auth/login', async () => {
    mockFetchResponse({ token: 'jwt', user: { id: '1' } });

    await apiService.login('alice@test.com', 'pass123');

    const [url, opts] = fetchSpy.mock.calls[0];
    expect(url).toContain('/auth/login');
    expect(opts.method).toBe('POST');
  });

  // ---- Conversations ----

  it('getConversations() should GET /conversations', async () => {
    mockFetchResponse([]);

    await apiService.getConversations();

    const [url] = fetchSpy.mock.calls[0];
    expect(url).toContain('/conversations');
  });

  it('createConversation() should POST with type, member_ids, title', async () => {
    mockFetchResponse({ id: 'conv-1' });

    await apiService.createConversation('dm', ['user-2'], null);

    const [, opts] = fetchSpy.mock.calls[0];
    const body = JSON.parse(opts.body);
    expect(body.type).toBe('dm');
    expect(body.member_ids).toEqual(['user-2']);
    expect(body.title).toBeNull();
  });

  // ---- Messages ----

  it('getMessages() should include pagination params', async () => {
    mockFetchResponse([]);

    await apiService.getMessages('conv-1', '2025-01-01T00:00:00Z', 25);

    const [url] = fetchSpy.mock.calls[0];
    expect(url).toContain('/conversations/conv-1/messages');
    expect(url).toContain('limit=25');
    expect(url).toContain('before=');
  });

  // ---- Call History ----

  it('getCallHistory() should include limit and offset', async () => {
    mockFetchResponse([]);
    localStorage.setItem('token', 'jwt');

    await apiService.getCallHistory(10, 5);

    const [url] = fetchSpy.mock.calls[0];
    expect(url).toContain('/calls?limit=10&offset=5');
  });

  it('getCall() should fetch specific call', async () => {
    mockFetchResponse({ id: 'call-1', status: 'ended' });
    localStorage.setItem('token', 'jwt');

    const result = await apiService.getCall('call-1');

    const [url] = fetchSpy.mock.calls[0];
    expect(url).toContain('/calls/call-1');
    expect(result.status).toBe('ended');
  });

  it('getMissedCallCount() should GET /calls/missed/count', async () => {
    mockFetchResponse({ count: 3 });
    localStorage.setItem('token', 'jwt');

    const result = await apiService.getMissedCallCount();
    expect(result.count).toBe(3);
  });

  it('createCall() should POST with conversation_id and call_type', async () => {
    mockFetchResponse({ id: 'call-2' });
    localStorage.setItem('token', 'jwt');

    await apiService.createCall('conv-1', 'audio');

    const [, opts] = fetchSpy.mock.calls[0];
    const body = JSON.parse(opts.body);
    expect(body.conversation_id).toBe('conv-1');
    expect(body.call_type).toBe('audio');
  });

  it('updateCall() should PATCH with status', async () => {
    mockFetchResponse({});
    localStorage.setItem('token', 'jwt');

    await apiService.updateCall('call-1', 'ended');

    const [url, opts] = fetchSpy.mock.calls[0];
    expect(url).toContain('/calls/call-1');
    expect(opts.method).toBe('PATCH');
    expect(JSON.parse(opts.body).status).toBe('ended');
  });

  // ---- Search ----

  it('searchMessages() should include query and limit', async () => {
    mockFetchResponse([]);

    await apiService.searchMessages('conv-1', 'hello world', 25);

    const [url] = fetchSpy.mock.calls[0];
    expect(url).toContain('search?q=hello%20world');
    expect(url).toContain('limit=25');
  });

  // ---- Starred / Archive ----

  it('starMessage() should POST to /messages/:id/star', async () => {
    mockFetchResponse({});
    localStorage.setItem('token', 'jwt');

    await apiService.starMessage('msg-1');

    const [url, opts] = fetchSpy.mock.calls[0];
    expect(url).toContain('/messages/msg-1/star');
    expect(opts.method).toBe('POST');
  });

  it('archiveConversation() should POST', async () => {
    mockFetchResponse({});
    localStorage.setItem('token', 'jwt');

    await apiService.archiveConversation('conv-1');

    const [url, opts] = fetchSpy.mock.calls[0];
    expect(url).toContain('/conversations/conv-1/archive');
    expect(opts.method).toBe('POST');
  });

  // ---- Network error ----

  it('should throw user-friendly message on network error', async () => {
    fetchSpy.mockRejectedValueOnce(new TypeError('Failed to fetch'));

    await expect(apiService.getConversations()).rejects.toThrow(
      'Network error'
    );
  });
});
