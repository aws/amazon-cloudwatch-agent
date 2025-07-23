/**
 * Safely save data to localStorage with error handling
 */
export const saveToLocalStorage = <T>(key: string, data: T): boolean => {
  try {
    const serializedData = JSON.stringify(data);
    localStorage.setItem(key, serializedData);
    return true;
  } catch (error) {
    console.error(`Failed to save to localStorage (key: ${key}):`, error);
    return false;
  }
};

/**
 * Safely load data from localStorage with error handling and default fallback
 */
export const loadFromLocalStorage = <T>(key: string, defaultValue: T): T => {
  try {
    const item = localStorage.getItem(key);
    if (item === null) {
      return defaultValue;
    }
    
    const parsedData = JSON.parse(item);
    
    // Additional validation for dates in ConfigTemplate objects
    if (Array.isArray(parsedData)) {
      return parsedData.map((item: any) => {
        if (item.createdAt && typeof item.createdAt === 'string') {
          item.createdAt = new Date(item.createdAt);
        }
        if (item.updatedAt && typeof item.updatedAt === 'string') {
          item.updatedAt = new Date(item.updatedAt);
        }
        return item;
      }) as T;
    }
    
    return parsedData;
  } catch (error) {
    console.error(`Failed to load from localStorage (key: ${key}):`, error);
    return defaultValue;
  }
};

/**
 * Remove item from localStorage
 */
export const removeFromLocalStorage = (key: string): boolean => {
  try {
    localStorage.removeItem(key);
    return true;
  } catch (error) {
    console.error(`Failed to remove from localStorage (key: ${key}):`, error);
    return false;
  }
};

/**
 * Clear all CloudWatch config generator data from localStorage
 */
export const clearAllConfigData = (): boolean => {
  try {
    const keysToRemove = [
      'cloudwatch-templates',
      'cloudwatch-current-config'
    ];
    
    keysToRemove.forEach(key => {
      localStorage.removeItem(key);
    });
    
    return true;
  } catch (error) {
    console.error('Failed to clear config data from localStorage:', error);
    return false;
  }
};

/**
 * Check if localStorage is available
 */
export const isLocalStorageAvailable = (): boolean => {
  try {
    const testKey = '__localStorage_test__';
    localStorage.setItem(testKey, 'test');
    localStorage.removeItem(testKey);
    return true;
  } catch {
    return false;
  }
};