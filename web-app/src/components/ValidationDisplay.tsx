import React from 'react';
import { ValidationError } from '../types/config';

interface ValidationDisplayProps {
  errors: ValidationError[];
  className?: string;
}

export const ValidationDisplay: React.FC<ValidationDisplayProps> = ({ 
  errors, 
  className = '' 
}) => {
  if (errors.length === 0) {
    return null;
  }

  return (
    <div className={`validation-errors ${className}`} role="alert" aria-live="polite">
      <div className="bg-red-50 border border-red-200 rounded-md p-4">
        <div className="flex">
          <div className="flex-shrink-0">
            <svg 
              className="h-5 w-5 text-red-400" 
              viewBox="0 0 20 20" 
              fill="currentColor"
              aria-hidden="true"
            >
              <path 
                fillRule="evenodd" 
                d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.28 7.22a.75.75 0 00-1.06 1.06L8.94 10l-1.72 1.72a.75.75 0 101.06 1.06L10 11.06l1.72 1.72a.75.75 0 101.06-1.06L11.06 10l1.72-1.72a.75.75 0 00-1.06-1.06L10 8.94 8.28 7.22z" 
                clipRule="evenodd" 
              />
            </svg>
          </div>
          <div className="ml-3">
            <h3 className="text-sm font-medium text-red-800">
              Configuration Errors ({errors.length})
            </h3>
            <div className="mt-2 text-sm text-red-700">
              <ul className="list-disc space-y-1 pl-5">
                {errors.map((error, index) => (
                  <li key={index}>
                    <strong>{error.field}:</strong> {error.message}
                    {error.path && (
                      <span className="text-xs text-red-600 block mt-1">
                        Path: {error.path}
                      </span>
                    )}
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

interface FieldValidationProps {
  errors: ValidationError[];
  fieldName: string;
  className?: string;
}

export const FieldValidation: React.FC<FieldValidationProps> = ({ 
  errors, 
  fieldName, 
  className = '' 
}) => {
  const fieldErrors = errors.filter(error => error.field === fieldName);
  
  if (fieldErrors.length === 0) {
    return null;
  }

  return (
    <div className={`field-validation ${className}`} role="alert">
      {fieldErrors.map((error, index) => (
        <p key={index} className="text-sm text-red-600 mt-1">
          {error.message}
        </p>
      ))}
    </div>
  );
};

interface ValidationSummaryProps {
  errors: ValidationError[];
  onFieldClick?: (fieldPath: string) => void;
}

export const ValidationSummary: React.FC<ValidationSummaryProps> = ({ 
  errors, 
  onFieldClick 
}) => {
  if (errors.length === 0) {
    return (
      <div className="bg-green-50 border border-green-200 rounded-md p-4">
        <div className="flex">
          <div className="flex-shrink-0">
            <svg 
              className="h-5 w-5 text-green-400" 
              viewBox="0 0 20 20" 
              fill="currentColor"
              aria-hidden="true"
            >
              <path 
                fillRule="evenodd" 
                d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.236 4.53L7.53 10.53a.75.75 0 00-1.06 1.06l2.25 2.25a.75.75 0 001.14-.094l3.75-5.25z" 
                clipRule="evenodd" 
              />
            </svg>
          </div>
          <div className="ml-3">
            <p className="text-sm font-medium text-green-800">
              Configuration is valid
            </p>
          </div>
        </div>
      </div>
    );
  }

  // Group errors by field
  const errorsByField = errors.reduce((acc, error) => {
    if (!acc[error.field]) {
      acc[error.field] = [];
    }
    acc[error.field].push(error);
    return acc;
  }, {} as Record<string, ValidationError[]>);

  return (
    <div className="validation-summary">
      <div className="bg-red-50 border border-red-200 rounded-md p-4">
        <div className="flex">
          <div className="flex-shrink-0">
            <svg 
              className="h-5 w-5 text-red-400" 
              viewBox="0 0 20 20" 
              fill="currentColor"
              aria-hidden="true"
            >
              <path 
                fillRule="evenodd" 
                d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.28 7.22a.75.75 0 00-1.06 1.06L8.94 10l-1.72 1.72a.75.75 0 101.06 1.06L10 11.06l1.72 1.72a.75.75 0 101.06-1.06L11.06 10l1.72-1.72a.75.75 0 00-1.06-1.06L10 8.94 8.28 7.22z" 
                clipRule="evenodd" 
              />
            </svg>
          </div>
          <div className="ml-3 w-full">
            <h3 className="text-sm font-medium text-red-800">
              Configuration has {errors.length} error{errors.length !== 1 ? 's' : ''}
            </h3>
            <div className="mt-4">
              <div className="space-y-3">
                {Object.entries(errorsByField).map(([field, fieldErrors]) => (
                  <div key={field} className="border-l-2 border-red-300 pl-3">
                    <button
                      type="button"
                      className="text-sm font-medium text-red-800 hover:text-red-900 focus:outline-none focus:underline"
                      onClick={() => onFieldClick?.(fieldErrors[0].path || '')}
                    >
                      {field} ({fieldErrors.length} error{fieldErrors.length !== 1 ? 's' : ''})
                    </button>
                    <ul className="mt-1 space-y-1">
                      {fieldErrors.map((error, index) => (
                        <li key={index} className="text-sm text-red-700">
                          • {error.message}
                        </li>
                      ))}
                    </ul>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

interface ValidationStatusProps {
  isValid: boolean;
  errorCount: number;
  className?: string;
}

export const ValidationStatus: React.FC<ValidationStatusProps> = ({ 
  isValid, 
  errorCount, 
  className = '' 
}) => {
  return (
    <div className={`validation-status ${className}`}>
      {isValid ? (
        <div className="flex items-center text-green-600">
          <svg 
            className="h-4 w-4 mr-2" 
            viewBox="0 0 20 20" 
            fill="currentColor"
            aria-hidden="true"
          >
            <path 
              fillRule="evenodd" 
              d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.236 4.53L7.53 10.53a.75.75 0 00-1.06 1.06l2.25 2.25a.75.75 0 001.14-.094l3.75-5.25z" 
              clipRule="evenodd" 
            />
          </svg>
          <span className="text-sm font-medium">Valid Configuration</span>
        </div>
      ) : (
        <div className="flex items-center text-red-600">
          <svg 
            className="h-4 w-4 mr-2" 
            viewBox="0 0 20 20" 
            fill="currentColor"
            aria-hidden="true"
          >
            <path 
              fillRule="evenodd" 
              d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.28 7.22a.75.75 0 00-1.06 1.06L8.94 10l-1.72 1.72a.75.75 0 101.06 1.06L10 11.06l1.72 1.72a.75.75 0 101.06-1.06L11.06 10l1.72-1.72a.75.75 0 00-1.06-1.06L10 8.94 8.28 7.22z" 
              clipRule="evenodd" 
            />
          </svg>
          <span className="text-sm font-medium">
            {errorCount} Error{errorCount !== 1 ? 's' : ''}
          </span>
        </div>
      )}
    </div>
  );
};