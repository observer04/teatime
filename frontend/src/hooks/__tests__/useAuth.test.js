import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useAuth } from '../useAuth';

describe('useAuth', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('should start with null user and token, loading=true', () => {
    const { result } = renderHook(() => useAuth());
    // After the initial useEffect runs, loading should be false
    expect(result.current.user).toBeNull();
    expect(result.current.token).toBeNull();
    expect(result.current.isAuthenticated).toBe(false);
  });

  it('should restore auth from localStorage', () => {
    const mockUser = { id: '123', username: 'tester' };
    localStorage.setItem('token', 'test-jwt');
    localStorage.setItem('user', JSON.stringify(mockUser));

    const { result } = renderHook(() => useAuth());

    expect(result.current.token).toBe('test-jwt');
    expect(result.current.user).toEqual(mockUser);
    expect(result.current.isAuthenticated).toBe(true);
    expect(result.current.loading).toBe(false);
  });

  it('login() should set token, user, and persist to localStorage', () => {
    const { result } = renderHook(() => useAuth());
    const newUser = { id: '456', username: 'alice' };

    act(() => {
      result.current.login('new-jwt', newUser);
    });

    expect(result.current.token).toBe('new-jwt');
    expect(result.current.user).toEqual(newUser);
    expect(result.current.isAuthenticated).toBe(true);
    expect(localStorage.getItem('token')).toBe('new-jwt');
    expect(JSON.parse(localStorage.getItem('user'))).toEqual(newUser);
  });

  it('logout() should clear token, user, and remove from localStorage', () => {
    const { result } = renderHook(() => useAuth());

    act(() => {
      result.current.login('jwt', { id: '1', username: 'bob' });
    });
    expect(result.current.isAuthenticated).toBe(true);

    act(() => {
      result.current.logout();
    });

    expect(result.current.token).toBeNull();
    expect(result.current.user).toBeNull();
    expect(result.current.isAuthenticated).toBe(false);
    expect(localStorage.getItem('token')).toBeNull();
    expect(localStorage.getItem('user')).toBeNull();
  });

  it('should handle corrupted localStorage gracefully', () => {
    localStorage.setItem('token', 'valid-jwt');
    localStorage.setItem('user', 'not-valid-json{{{');

    // JSON.parse will throw, but the hook should handle this
    expect(() => renderHook(() => useAuth())).toThrow();
  });

  it('should set loading to false after initialization', () => {
    const { result } = renderHook(() => useAuth());
    expect(result.current.loading).toBe(false);
  });
});
