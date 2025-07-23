import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  saveToLocalStorage,
  loadFromLocalStorage,
  removeFromLocalStorage,
  clearAllConfigData,
  isLocalStorageAvailable
} from '../localStorage';
import { ConfigTemplate } from '../../types/config';

// Mock console.error to avoid noise in tests
const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

describe('localStorage utilities', () => {
  beforeEach(() => {
    consoleSpy.mockClear();
  });

  describe('saveToLocalStorage', () => {
    it('should save data to localStorage', () => {
      const testData = { key: 'value', number: 42 };
      const result = saveToLocalStorage('test-key', testData);
      
      expect(result).toBe(true);
      expect(localStorage.setItem).toHaveBeenCalledWith('test-key', JSON.stringify(testData));
    });

    it('should handle localStorage errors gracefully', () => {
      const mockError = new Error('Storage quota exceeded');
      vi.mocked(localStorage.setItem).mockImplementationOnce(() => {
        throw mockError;
      });

      const result = saveToLocalStorage('test-key', { data: 'test' });
      
      expect(result).toBe(false);
      expect(consoleSpy).toHaveBeenCalledWith(
        'Failed to save to localStorage (key: test-key):',
        mockError
      );
    });
  });

  describe('loadFromLocalStorage', () => {
    it('should load data from localStorage', () => {
      const testData = { key: 'value', number: 42 };
      vi.mocked(localStorage.getItem).mockReturnValue(JSON.stringify(testData));

      const result = loadFromLocalStorage('test-key', {});
      
      expect(result).toEqual(testData);
      expect(localStorage.getItem).toHaveBeenCalledWith('test-key');
    });

    it('should return default value when item does not exist', () => {
      vi.mocked(localStorage.getItem).mockReturnValue(null);
      const defaultValue = { default: true };

      const result = loadFromLocalStorage('non-existent-key', defaultValue);
      
      expect(result).toEqual(defaultValue);
    });

    it('should handle JSON parsing errors gracefully', () => {
      vi.mocked(localStorage.getItem).mockReturnValue('invalid json');
      const defaultValue = { default: true };

      const result = loadFromLocalStorage('test-key', defaultValue);
      
      expect(result).toEqual(defaultValue);
      expect(consoleSpy).toHaveBeenCalledWith(
        'Failed to load from localStorage (key: test-key):',
        expect.any(SyntaxError)
      );
    });

    it('should parse dates in ConfigTemplate arrays', () => {
      const templateData = [
        {
          id: 'template-1',
          name: 'Test Template',
          createdAt: '2023-01-01T00:00:00.000Z',
          updatedAt: '2023-01-02T00:00:00.000Z',
          operatingSystem: 'linux',
          configuration: {}
        }
      ];

      vi.mocked(localStorage.getItem).mockReturnValue(JSON.stringify(templateData));

      const result = loadFromLocalStorage<ConfigTemplate[]>('templates', []);
      
      expect(result[0].createdAt).toBeInstanceOf(Date);
      expect(result[0].updatedAt).toBeInstanceOf(Date);
      expect(result[0].createdAt.toISOString()).toBe('2023-01-01T00:00:00.000Z');
      expect(result[0].updatedAt.toISOString()).toBe('2023-01-02T00:00:00.000Z');
    });
  });

  describe('removeFromLocalStorage', () => {
    it('should remove item from localStorage', () => {
      const result = removeFromLocalStorage('test-key');
      
      expect(result).toBe(true);
      expect(localStorage.removeItem).toHaveBeenCalledWith('test-key');
    });

    it('should handle removal errors gracefully', () => {
      const mockError = new Error('Access denied');
      vi.mocked(localStorage.removeItem).mockImplementationOnce(() => {
        throw mockError;
      });

      const result = removeFromLocalStorage('test-key');
      
      expect(result).toBe(false);
      expect(consoleSpy).toHaveBeenCalledWith(
        'Failed to remove from localStorage (key: test-key):',
        mockError
      );
    });
  });

  describe('clearAllConfigData', () => {
    it('should clear all config-related data', () => {
      const result = clearAllConfigData();
      
      expect(result).toBe(true);
      expect(localStorage.removeItem).toHaveBeenCalledWith('cloudwatch-templates');
      expect(localStorage.removeItem).toHaveBeenCalledWith('cloudwatch-current-config');
    });

    it('should handle clearing errors gracefully', () => {
      const mockError = new Error('Access denied');
      vi.mocked(localStorage.removeItem).mockImplementationOnce(() => {
        throw mockError;
      });

      const result = clearAllConfigData();
      
      expect(result).toBe(false);
      expect(consoleSpy).toHaveBeenCalledWith(
        'Failed to clear config data from localStorage:',
        mockError
      );
    });
  });

  describe('isLocalStorageAvailable', () => {
    it('should return true when localStorage is available', () => {
      const result = isLocalStorageAvailable();
      
      expect(result).toBe(true);
      expect(localStorage.setItem).toHaveBeenCalledWith('__localStorage_test__', 'test');
      expect(localStorage.removeItem).toHaveBeenCalledWith('__localStorage_test__');
    });

    it('should return false when localStorage is not available', () => {
      vi.mocked(localStorage.setItem).mockImplementationOnce(() => {
        throw new Error('localStorage not available');
      });

      const result = isLocalStorageAvailable();
      
      expect(result).toBe(false);
    });
  });
});