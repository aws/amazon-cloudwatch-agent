import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { 
  ValidationDisplay, 
  FieldValidation, 
  ValidationSummary, 
  ValidationStatus 
} from '../ValidationDisplay';
import { ValidationError } from '../../types/config';

describe('ValidationDisplay', () => {
  const mockErrors: ValidationError[] = [
    { field: 'CPU', message: 'Invalid measurement', path: '/metrics/cpu' },
    { field: 'Region', message: 'Invalid format', path: '/agent/region' }
  ];

  it('should render nothing when no errors', () => {
    const { container } = render(<ValidationDisplay errors={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it('should render errors with proper structure', () => {
    render(<ValidationDisplay errors={mockErrors} />);
    
    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.getByText('Configuration Errors (2)')).toBeInTheDocument();
    expect(screen.getByText(/CPU:/)).toBeInTheDocument();
    expect(screen.getByText('Invalid measurement')).toBeInTheDocument();
    expect(screen.getByText(/Region:/)).toBeInTheDocument();
    expect(screen.getByText('Invalid format')).toBeInTheDocument();
  });

  it('should display error paths when available', () => {
    render(<ValidationDisplay errors={mockErrors} />);
    
    expect(screen.getByText('Path: /metrics/cpu')).toBeInTheDocument();
    expect(screen.getByText('Path: /agent/region')).toBeInTheDocument();
  });

  it('should apply custom className', () => {
    const { container } = render(
      <ValidationDisplay errors={mockErrors} className="custom-class" />
    );
    
    expect(container.firstChild).toHaveClass('custom-class');
  });

  it('should have proper accessibility attributes', () => {
    render(<ValidationDisplay errors={mockErrors} />);
    
    const alert = screen.getByRole('alert');
    expect(alert).toHaveAttribute('aria-live', 'polite');
  });
});

describe('FieldValidation', () => {
  const mockErrors: ValidationError[] = [
    { field: 'CPU', message: 'Invalid measurement', path: '/metrics/cpu' },
    { field: 'Region', message: 'Invalid format', path: '/agent/region' },
    { field: 'CPU', message: 'Required field', path: '/metrics/cpu' }
  ];

  it('should render nothing when no matching field errors', () => {
    const { container } = render(
      <FieldValidation errors={mockErrors} fieldName="Memory" />
    );
    expect(container.firstChild).toBeNull();
  });

  it('should render errors for specific field', () => {
    render(<FieldValidation errors={mockErrors} fieldName="CPU" />);
    
    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.getByText('Invalid measurement')).toBeInTheDocument();
    expect(screen.getByText('Required field')).toBeInTheDocument();
  });

  it('should not render errors for other fields', () => {
    render(<FieldValidation errors={mockErrors} fieldName="CPU" />);
    
    expect(screen.queryByText('Invalid format')).not.toBeInTheDocument();
  });

  it('should apply custom className', () => {
    const { container } = render(
      <FieldValidation 
        errors={mockErrors} 
        fieldName="CPU" 
        className="custom-field-class" 
      />
    );
    
    expect(container.firstChild).toHaveClass('custom-field-class');
  });
});

describe('ValidationSummary', () => {
  const mockErrors: ValidationError[] = [
    { field: 'CPU', message: 'Invalid measurement', path: '/metrics/cpu' },
    { field: 'CPU', message: 'Required field', path: '/metrics/cpu' },
    { field: 'Region', message: 'Invalid format', path: '/agent/region' }
  ];

  it('should render success state when no errors', () => {
    render(<ValidationSummary errors={[]} />);
    
    expect(screen.getByText('Configuration is valid')).toBeInTheDocument();
    expect(screen.getByRole('generic')).toHaveClass('bg-green-50');
  });

  it('should render error summary with grouped errors', () => {
    render(<ValidationSummary errors={mockErrors} />);
    
    expect(screen.getByText('Configuration has 3 errors')).toBeInTheDocument();
    expect(screen.getByText('CPU (2 errors)')).toBeInTheDocument();
    expect(screen.getByText('Region (1 error)')).toBeInTheDocument();
  });

  it('should handle singular error count', () => {
    const singleError = [mockErrors[0]];
    render(<ValidationSummary errors={singleError} />);
    
    expect(screen.getByText('Configuration has 1 error')).toBeInTheDocument();
  });

  it('should call onFieldClick when field button is clicked', () => {
    const mockOnFieldClick = vi.fn();
    render(
      <ValidationSummary errors={mockErrors} onFieldClick={mockOnFieldClick} />
    );
    
    const cpuButton = screen.getByText('CPU (2 errors)');
    fireEvent.click(cpuButton);
    
    expect(mockOnFieldClick).toHaveBeenCalledWith('/metrics/cpu');
  });

  it('should display individual error messages', () => {
    render(<ValidationSummary errors={mockErrors} />);
    
    expect(screen.getByText('• Invalid measurement')).toBeInTheDocument();
    expect(screen.getByText('• Required field')).toBeInTheDocument();
    expect(screen.getByText('• Invalid format')).toBeInTheDocument();
  });
});

describe('ValidationStatus', () => {
  it('should render valid status', () => {
    render(<ValidationStatus isValid={true} errorCount={0} />);
    
    expect(screen.getByText('Valid Configuration')).toBeInTheDocument();
    expect(screen.getByRole('generic')).toHaveClass('text-green-600');
  });

  it('should render invalid status with error count', () => {
    render(<ValidationStatus isValid={false} errorCount={3} />);
    
    expect(screen.getByText('3 Errors')).toBeInTheDocument();
    expect(screen.getByRole('generic')).toHaveClass('text-red-600');
  });

  it('should handle singular error count', () => {
    render(<ValidationStatus isValid={false} errorCount={1} />);
    
    expect(screen.getByText('1 Error')).toBeInTheDocument();
  });

  it('should apply custom className', () => {
    const { container } = render(
      <ValidationStatus 
        isValid={true} 
        errorCount={0} 
        className="custom-status-class" 
      />
    );
    
    expect(container.firstChild).toHaveClass('custom-status-class');
  });

  it('should have proper SVG icons', () => {
    const { rerender } = render(
      <ValidationStatus isValid={true} errorCount={0} />
    );
    
    // Valid state should have checkmark icon
    expect(screen.getByRole('generic')).toContainHTML('svg');
    
    rerender(<ValidationStatus isValid={false} errorCount={1} />);
    
    // Invalid state should have error icon
    expect(screen.getByRole('generic')).toContainHTML('svg');
  });
});

describe('Accessibility', () => {
  const mockErrors: ValidationError[] = [
    { field: 'CPU', message: 'Invalid measurement', path: '/metrics/cpu' }
  ];

  it('should have proper ARIA attributes for ValidationDisplay', () => {
    render(<ValidationDisplay errors={mockErrors} />);
    
    const alert = screen.getByRole('alert');
    expect(alert).toHaveAttribute('aria-live', 'polite');
  });

  it('should have proper ARIA attributes for FieldValidation', () => {
    render(<FieldValidation errors={mockErrors} fieldName="CPU" />);
    
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('should have focusable buttons in ValidationSummary', () => {
    const mockOnFieldClick = vi.fn();
    render(
      <ValidationSummary errors={mockErrors} onFieldClick={mockOnFieldClick} />
    );
    
    const button = screen.getByRole('button');
    expect(button).toHaveAttribute('type', 'button');
    expect(button).toHaveClass('focus:outline-none', 'focus:underline');
  });

  it('should have proper semantic structure', () => {
    render(<ValidationDisplay errors={mockErrors} />);
    
    // Should have proper heading structure
    expect(screen.getByRole('heading', { level: 3 })).toBeInTheDocument();
    
    // Should have proper list structure
    expect(screen.getByRole('list')).toBeInTheDocument();
    expect(screen.getAllByRole('listitem')).toHaveLength(1);
  });
});