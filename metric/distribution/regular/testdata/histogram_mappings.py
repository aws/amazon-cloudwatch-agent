import json
import math
import matplotlib.pyplot as plt
import numpy as np
import os
import pdb

from pathlib import Path
from typing import Dict, List, Tuple

def plot_original_histogram(data, ax, title: str, color: str):
    """Plot original histogram using exact bucket boundaries."""
    boundaries = data.get('Boundaries', [])
    counts = data['Counts']
    min_val = data.get('Min')
    max_val = data.get('Max')
    total_count = sum(counts)
    
    # Handle case with no boundaries (single bucket)
    if not boundaries or len(boundaries) == 0:
        if min_val is not None and max_val is not None:
            left_edges = [min_val]
            widths = [max_val - min_val]
        else:
            # Use arbitrary range if no min/max
            left_edges = [-10]
            widths = [20]
    else:
        # Calculate exact bucket edges and widths
        left_edges = []
        widths = []
        
        for i in range(len(counts)):
            if i == 0:
                # First bucket: from min to first boundary
                left = min_val if min_val is not None else boundaries[0] - (boundaries[1] - boundaries[0]) if len(boundaries) > 1 else boundaries[0] - 10
                right = boundaries[0]
            elif i == len(counts) - 1:
                # Last bucket: from last boundary to max
                left = boundaries[i-1]
                right = max_val if max_val is not None else boundaries[i-1] + (boundaries[i-1] - boundaries[i-2]) if len(boundaries) > 1 else boundaries[i-1] + 10
            else:
                # Middle buckets: between boundaries
                left = boundaries[i-1]
                right = boundaries[i]
            
            left_edges.append(left)
            widths.append(right - left)
    
    ax.bar(left_edges, counts, width=widths, alpha=0.7, edgecolor='black', linewidth=0.8, color=color, align='edge')
    ax.set_title(f'{title} (Count: {total_count})')
    ax.set_ylabel('Counts')
    ax.grid(True, alpha=0.3)

def plot_cw_histogram_bars(histogram: Dict[float, float], histogram_min: float, histogram_max: float, ax, title: str, color: str):
    """Plot histogram bars on given axes."""
    values = sorted(histogram.keys())
    counts = [histogram[v] for v in values]
    total_count = sum(counts)
    
    if len(values) == 1:
        # Single bar case
        width = (histogram_max - histogram_min) * 0.8
        ax.bar(values, counts, width=width, alpha=0.7, edgecolor='black', linewidth=1.5, color=color)
    else:
        # Calculate minimum gap to prevent overlaps
        gaps = [values[i+1] - values[i] for i in range(len(values)-1)]
        min_gap = min(gaps)
        max_width = min_gap * 0.8  # Use 80% of minimum gap
        
        widths = []
        for i in range(len(values)):
            if i == 0:
                # First bar: extend to histogram_min or use half-gap to next
                left_space = values[0] - histogram_min
                right_space = (values[1] - values[0]) / 2 if len(values) > 1 else (histogram_max - values[0])
                width = min(left_space + right_space, max_width)
            elif i == len(values) - 1:
                # Last bar: extend to histogram_max or use half-gap from previous
                left_space = (values[i] - values[i-1]) / 2
                right_space = histogram_max - values[i]
                width = min(left_space + right_space, max_width)
            else:
                # Middle bars: use half-gaps on both sides
                left_space = (values[i] - values[i-1]) / 2
                right_space = (values[i+1] - values[i]) / 2
                width = min(left_space + right_space, max_width)
            
            widths.append(width)
        
        ax.bar(values, counts, width=widths, alpha=0.7, edgecolor='black', linewidth=0.8, color=color)
    
    ax.scatter(values, counts, color='red', s=50, zorder=5)
    ax.set_title(f'{title} (Sum: {total_count})')
    ax.set_ylabel('Counts')
    ax.grid(True, alpha=0.3)

def load_json_data(filepath):
    """Load histogram data from JSON file."""
    with open(filepath, 'r') as f:
        data = json.load(f)
    return data['values'], data['counts']

def load_original_histogram(filepath):
    """Load original histogram format."""
    with open(filepath, 'r') as f:
        data = json.load(f)
    return data

def plot_all_folders_comparison(json_filename):
    """Plot the same JSON file from all folders for comparison."""
    base_path = Path('.')
    folders = ['original', 'cwagent', 'even', 'middlepoint', 'exponential', 'exponentialcw']
    colors = ['black', 'green', 'orange', 'red', 'purple', 'blue']
    
    fig, ax = plt.subplots(len(folders), 1, figsize=(12, 20))
    
    i = -1
    for folder, color in zip(folders, colors):
        i += 1
        filepath = base_path / folder / (json_filename+".json")
        if filepath.exists():
            try:
                if folder == 'original':
                    data = load_original_histogram(filepath)
                    plot_original_histogram(data, ax[i], f'{folder.capitalize()} Mapping', color)
                else:
                    values, counts = load_json_data(filepath)
                    if not values:  # Skip if no values
                        continue
                    hist = {values[j]: counts[j] for j in range(len(values))}
                    plot_cw_histogram_bars(hist, min(values), max(values), ax[i], f'{folder.capitalize()} Mapping', color)
            except Exception as e:
                print(f"Error processing {filepath}: {e}")
    
    plt.tight_layout()
    plt.savefig(f"comparisons/{json_filename}.png", dpi=300, bbox_inches='tight')
    plt.show()

# Example usage
if __name__ == "__main__":
    # Get all JSON files from original folder
    original_path = Path('./original')
    if original_path.exists():
        json_files = [f.stem for f in original_path.iterdir() if f.suffix == '.json']
        
        # Plot comparison for each JSON file
        for json_file in json_files:
            print(f"Processing {json_file}...")
            plot_all_folders_comparison(json_file)
    else:
        print("Original folder not found. Using sample data.")
        # Fallback to sample data
        bounds = [0.3, 0.5, 0.7, 0.9, 1.1]
        bucket_counts = [80, 120, 150, 130, 70]
        histogram_min, histogram_max = 0.2, 1.2
        visualize_all_mappings(bounds, bucket_counts, histogram_min, histogram_max)
