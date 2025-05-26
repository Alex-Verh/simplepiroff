import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns
import os
import numpy as np
import argparse

# Set the style
sns.set(style="whitegrid")
plt.rcParams.update({'font.size': 12})

def fix_csv_data(df):
    """Fix potential issues with the CSV data"""
    # Print detailed info for debugging
    print(f"DataFrame shape before cleaning: {df.shape}")
    
    # Check for completely empty columns and drop them
    empty_cols = [col for col in df.columns if df[col].isna().all()]
    if empty_cols:
        print(f"Dropping empty columns: {empty_cols}")
        df = df.drop(columns=empty_cols)
    
    # Fix swapped column values
    if 'db_size' in df.columns and 'record_size' in df.columns:
        # Define expected ranges
        db_size_expected = [1, 10, 100, 1000, 10000, 100000, 1000000]
        record_size_expected = [8, 16, 32, 64, 128, 256, 512, 1024]
        
        # First convert to numeric if possible
        for col in ['db_size', 'record_size']:
            if col in df.columns:
                if df[col].dtype == 'object':
                    df[col] = pd.to_numeric(df[col], errors='ignore')
        
        # Detect rows where values seem swapped
        for idx, row in df.iterrows():
            if pd.notna(row['db_size']) and pd.notna(row['record_size']):
                if isinstance(row['db_size'], (int, float)) and isinstance(row['record_size'], (int, float)):
                    # If db_size is in record_size range and record_size is in db_size range
                    if (row['db_size'] in record_size_expected and 
                        row['record_size'] in db_size_expected):
                        print(f"Fixing swapped values at row {idx}: db_size={row['db_size']}, record_size={row['record_size']}")
                        # Swap the values
                        temp = df.at[idx, 'db_size']
                        df.at[idx, 'db_size'] = df.at[idx, 'record_size']
                        df.at[idx, 'record_size'] = temp
    
    # Handle different column names and formats
    numeric_cols = ['setup_time', 'query_time', 'answer_time', 
                   'reconstruct_time', 'offline_download', 
                   'online_upload', 'online_download', 'run_count', 'run_id']
    
    # Convert to numeric, coercing errors
    for col in numeric_cols:
        if col in df.columns:
            df[col] = pd.to_numeric(df[col], errors='coerce')
    
    # Convert size columns to numeric
    if 'db_size' in df.columns:
        # Handle 'auto' values
        auto_mask = df['db_size'] == 'auto'
        if auto_mask.any():
            non_auto = df.loc[~auto_mask, 'db_size']
            if len(non_auto) > 0:
                median_value = pd.to_numeric(non_auto, errors='coerce').median()
                df.loc[auto_mask, 'db_size'] = median_value
            else:
                df.loc[auto_mask, 'db_size'] = 1000  # Default value if all are 'auto'
        df['db_size'] = pd.to_numeric(df['db_size'], errors='coerce')
    
    if 'record_size' in df.columns:
        # Handle 'auto' values
        auto_mask = df['record_size'] == 'auto'
        if auto_mask.any():
            non_auto = df.loc[~auto_mask, 'record_size']
            if len(non_auto) > 0:
                median_value = pd.to_numeric(non_auto, errors='coerce').median()
                df.loc[auto_mask, 'record_size'] = median_value
            else:
                df.loc[auto_mask, 'record_size'] = 32  # Default value if all are 'auto'
        df['record_size'] = pd.to_numeric(df['record_size'], errors='coerce')

    # Drop rows with all NaN values
    df = df.dropna(how='all')
    
    # Print column info after cleaning
    print(f"DataFrame shape after cleaning: {df.shape}")
    print(f"Column types: {df.dtypes}")
    
    # Clean up any remaining NaN values by replacing with column means
    for col in df.select_dtypes(include=['float64', 'int64']).columns:
        if df[col].isna().any():
            df[col] = df[col].fillna(df[col].mean())
    
    return df

def generate_plots(csv_path, output_dir="plots"):
    """Generate plots from any CSV results file"""
    if not os.path.exists(csv_path):
        print(f"Error: {csv_path} not found")
        return False
    
    # Determine the plot type from filename
    filename = os.path.basename(csv_path)
    plot_type = filename.split('_results.csv')[0]
    
    # Create plots directory
    os.makedirs(output_dir, exist_ok=True)
    
    try:
        # Load data - use error_bad_lines=False to skip problematic rows
        print(f"Loading data from {csv_path}...")
        # First try to detect the delimiter and number of columns
        with open(csv_path, 'r') as f:
            first_line = f.readline().strip()
            if ',' in first_line:
                delimiter = ','
            elif ';' in first_line:
                delimiter = ';'
            elif '\t' in first_line:
                delimiter = '\t'
            else:
                delimiter = ','  # Default to comma
        
        # Try to read the CSV with pandas, handling inconsistent rows
        try:
            # For newer pandas versions
            df = pd.read_csv(csv_path, delimiter=delimiter, on_bad_lines='skip')
        except TypeError:
            # For older pandas versions
            df = pd.read_csv(csv_path, delimiter=delimiter, error_bad_lines=False)
        
        # Print the column names for debugging
        print(f"Columns in CSV: {df.columns.tolist()}")
        
        # Fix data format issues
        df = fix_csv_data(df)
        
        # Determine if this is average data from the filename
        is_avg_data = "_avg" in plot_type
        
        # Generate different plots based on the data available
        if is_avg_data:
            plot_average_data(df, plot_type, output_dir)
        elif plot_type == "dbsize" or plot_type == "dbsize_runs":
            plot_dbsize_data(df, output_dir)
        elif plot_type == "recordsize" or plot_type == "recordsize_runs":
            plot_recordsize_data(df, output_dir)
        elif plot_type == "db_recordsize" or plot_type == "db_recordsize_runs":
            plot_combination_data(df, output_dir)
        else:
            # Generic plots
            plot_generic_data(df, plot_type, output_dir)
        
        return True
    
    except Exception as e:
        print(f"Error processing {csv_path}: {e}")
        
        # Try a more robust approach for severely malformed CSV files
        print("Attempting to read file with a more robust method...")
        try:
            # Read the file line by line and manually parse
            rows = []
            with open(csv_path, 'r') as f:
                for line in f:
                    if line.strip():  # Skip empty lines
                        values = [v.strip() for v in line.split(delimiter)]
                        rows.append(values)
            
            if not rows:
                print("No valid data found in the file")
                return False
                
            # Get headers from the first row
            headers = rows[0]
            
            # Create a list of dictionaries
            data = []
            max_cols = max(len(row) for row in rows)
            
            # Ensure headers has enough columns
            while len(headers) < max_cols:
                headers.append(f"column_{len(headers)}")
            
            # Create data dictionaries, padding any short rows
            for row in rows[1:]:
                row_dict = {}
                for i, value in enumerate(row):
                    if i < len(headers):
                        row_dict[headers[i]] = value
                data.append(row_dict)
            
            # Create DataFrame
            df = pd.DataFrame(data)
            
            # Fix data format issues
            df = fix_csv_data(df)
            
            # Determine if this is average data from the filename
            is_avg_data = "_avg" in plot_type
            
            # Generate plots with the recovered data
            if is_avg_data:
                plot_average_data(df, plot_type, output_dir)
            elif plot_type == "dbsize" or plot_type == "dbsize_runs":
                plot_dbsize_data(df, output_dir)
            elif plot_type == "recordsize" or plot_type == "recordsize_runs":
                plot_recordsize_data(df, output_dir)
            elif plot_type == "db_recordsize" or plot_type == "db_recordsize_runs":
                plot_combination_data(df, output_dir)
            else:
                plot_generic_data(df, plot_type, output_dir)
                
            return True
            
        except Exception as e2:
            print(f"Failed to parse {csv_path} with robust method: {e2}")
            return False

def plot_average_data(df, plot_type, output_dir):
    """Plot data that has been averaged over multiple runs"""
    # Check if we have run_count in the data
    has_run_count = 'run_count' in df.columns
    
    if not has_run_count:
        print("No run_count column found, treating as single-run data")
        if plot_type.startswith("dbsize"):
            plot_dbsize_data(df, output_dir)
        elif plot_type.startswith("recordsize"):
            plot_recordsize_data(df, output_dir)
        elif plot_type.startswith("db_recordsize"):
            plot_combination_data(df, output_dir)
        else:
            plot_generic_data(df, plot_type, output_dir)
        return
    
    # Get the run count (should be the same for all rows)
    run_count = int(df['run_count'].iloc[0])
    print(f"Plotting averaged data with {run_count} runs per data point")
    
    # Handle each type of plot
    if plot_type.startswith("dbsize"):
        plot_avg_dbsize_data(df, output_dir, run_count)
    elif plot_type.startswith("recordsize"):
        plot_avg_recordsize_data(df, output_dir, run_count)
    elif plot_type.startswith("db_recordsize"):
        plot_avg_combination_data(df, output_dir, run_count)
    else:
        plot_generic_data(df, plot_type, output_dir)  # Fall back to generic plotting

def plot_avg_dbsize_data(df, output_dir, run_count):
    """Plot averaged database size results"""
    # Sort by db_size for better visualization
    df = df.sort_values('db_size')
    
    # Plot 1: PIR Operation Times with run count in title
    plt.figure(figsize=(12, 8))
    
    time_columns = [col for col in ['setup_time', 'query_time', 'answer_time', 'reconstruct_time'] 
                   if col in df.columns]
    markers = ['o', 's', '^', 'd']
    
    for i, col in enumerate(time_columns):
        marker = markers[i % len(markers)]
        plt.plot(df['db_size'], df[col], marker=marker, linewidth=2, 
                label=col.replace('_', ' ').title())
    
    plt.xscale('log')
    plt.yscale('log')
    plt.xlabel('Database Size (entries)')
    plt.ylabel('Time (ms) - Average of multiple runs')
    plt.title(f'PIR Operation Times vs Database Size (Avg. of {run_count} runs)')
    plt.legend()
    plt.grid(True, which="both", ls="-", alpha=0.2)
    plt.tight_layout()
    
    plt.savefig(f'{output_dir}/dbsize_avg_times.png', dpi=300)
    print(f"Created plot: {output_dir}/dbsize_avg_times.png")
    
    # Plot 2: Stacked Bar Chart (if we have all needed columns)
    if all(col in df.columns for col in ['setup_time', 'query_time', 'answer_time', 'reconstruct_time']):
        plt.figure(figsize=(12, 8))
        operations = ['setup_time', 'query_time', 'answer_time', 'reconstruct_time']
        labels = ['Setup', 'Query Building', 'Query Answering', 'Reconstruction']
        colors = ['#1f77b4', '#ff7f0e', '#2ca02c', '#d62728']
        
        bottom = np.zeros(len(df))
        for i, col in enumerate(operations):
            plt.bar(df['db_size'], df[col], bottom=bottom, label=labels[i], color=colors[i])
            bottom += df[col].values
        
        plt.xscale('log')
        plt.xlabel('Database Size (entries)')
        plt.ylabel('Time (ms) - Average of multiple runs')
        plt.title(f'PIR Operation Time Breakdown (Avg. of {run_count} runs)')
        plt.legend()
        plt.grid(True, which="both", ls="-", alpha=0.2)
        plt.tight_layout()
        
        plt.savefig(f'{output_dir}/dbsize_avg_time_breakdown.png', dpi=300)
        print(f"Created plot: {output_dir}/dbsize_avg_time_breakdown.png")
    
    # Plot 3: Network usage (if available)
    network_cols = [col for col in ['offline_download', 'online_upload', 'online_download'] if col in df.columns]
    if network_cols:
        plt.figure(figsize=(12, 8))
        markers = ['o', 's', '^']
        
        for i, col in enumerate(network_cols):
            marker = markers[i % len(markers)]
            plt.plot(df['db_size'], df[col], marker=marker, linewidth=2, 
                     label=col.replace('_', ' ').title())
        
        plt.xscale('log')
        plt.yscale('log')
        plt.xlabel('Database Size (entries)')
        plt.ylabel('Data Transfer (KB) - Average of multiple runs')
        plt.title(f'PIR Network Usage vs Database Size (Avg. of {run_count} runs)')
        plt.legend()
        plt.grid(True, which="both", ls="-", alpha=0.2)
        plt.tight_layout()
        
        plt.savefig(f'{output_dir}/dbsize_avg_network.png', dpi=300)
        print(f"Created plot: {output_dir}/dbsize_avg_network.png")

def plot_avg_recordsize_data(df, output_dir, run_count):
    """Plot averaged record size results"""
    # Sort by record_size for better visualization
    df = df.sort_values('record_size')
    
    # Plot 1: PIR Operation Times
    plt.figure(figsize=(12, 8))
    
    time_columns = [col for col in ['setup_time', 'query_time', 'answer_time', 'reconstruct_time'] 
                   if col in df.columns]
    markers = ['o', 's', '^', 'd']
    
    for i, col in enumerate(time_columns):
        marker = markers[i % len(markers)]
        plt.plot(df['record_size'], df[col], marker=marker, linewidth=2, 
                label=col.replace('_', ' ').title())
    
    plt.xscale('log', base=2)  # Record sizes are typically powers of 2
    plt.yscale('log')
    plt.xlabel('Record Size (bits)')
    plt.ylabel('Time (ms) - Average of multiple runs')
    plt.title(f'PIR Operation Times vs Record Size (Avg. of {run_count} runs)')
    plt.legend()
    plt.grid(True, which="both", ls="-", alpha=0.2)
    plt.tight_layout()
    
    plt.savefig(f'{output_dir}/recordsize_avg_times.png', dpi=300)
    print(f"Created plot: {output_dir}/recordsize_avg_times.png")
    
    # Plot 2: Stacked Bar Chart (if we have all needed columns)
    if all(col in df.columns for col in ['setup_time', 'query_time', 'answer_time', 'reconstruct_time']):
        plt.figure(figsize=(12, 8))
        operations = ['setup_time', 'query_time', 'answer_time', 'reconstruct_time']
        labels = ['Setup', 'Query Building', 'Query Answering', 'Reconstruction']
        colors = ['#1f77b4', '#ff7f0e', '#2ca02c', '#d62728']
        
        bottom = np.zeros(len(df))
        for i, col in enumerate(operations):
            plt.bar(df['record_size'], df[col], bottom=bottom, label=labels[i], color=colors[i])
            bottom += df[col].values
        
        plt.xscale('log', base=2)
        plt.xlabel('Record Size (bits)')
        plt.ylabel('Time (ms) - Average of multiple runs')
        plt.title(f'PIR Operation Time Breakdown (Avg. of {run_count} runs)')
        plt.legend()
        plt.grid(True, which="both", ls="-", alpha=0.2)
        plt.tight_layout()
        
        plt.savefig(f'{output_dir}/recordsize_avg_time_breakdown.png', dpi=300)
        print(f"Created plot: {output_dir}/recordsize_avg_time_breakdown.png")

def plot_avg_combination_data(df, output_dir, run_count):
    """Create heatmaps for combination test results with averaged data"""
    # Check if we have enough data for heatmaps
    if len(df['db_size'].unique()) < 3 or len(df['record_size'].unique()) < 3:
        print("Not enough unique values for heatmaps. Creating regular plots instead.")
        plot_generic_data(df, "db_recordsize_avg", output_dir)
        return
        
    # Create heatmaps for each metric
    time_columns = [col for col in ['setup_time', 'query_time', 'answer_time', 'reconstruct_time'] 
                   if col in df.columns]
    
    for col in time_columns:
        try:
            pivot_df = df.pivot(index="db_size", columns="record_size", values=col)
            
            plt.figure(figsize=(12, 10))
            sns.heatmap(pivot_df, annot=True, fmt=".2f", cmap="viridis", 
                         cbar_kws={'label': f'Time (ms) - Avg. of {run_count} runs'})
            
            plt.title(f'{col.replace("_", " ").title()} for Different DB and Record Sizes (Avg. of {run_count} runs)')
            plt.tight_layout()
            
            plt.savefig(f'{output_dir}/heatmap_avg_{col}.png', dpi=300)
            print(f"Created heatmap: {output_dir}/heatmap_avg_{col}.png")
        except Exception as e:
            print(f"Error creating heatmap for {col}: {e}")
    
    # 3D visualization if matplotlib has mplot3d
    try:
        from mpl_toolkits.mplot3d import Axes3D
        
        for col in time_columns:
            fig = plt.figure(figsize=(12, 10))
            ax = fig.add_subplot(111, projection='3d')
            
            x = df['db_size']
            y = df['record_size']
            z = df[col]
            
            surf = ax.plot_trisurf(np.log10(x), np.log2(y), z, cmap='viridis', 
                                 edgecolor='none', alpha=0.8)
            
            ax.set_xlabel('Database Size (log10)')
            ax.set_ylabel('Record Size (log2)')
            ax.set_zlabel(f'{col.replace("_", " ").title()} (ms) - Avg. of {run_count} runs')
            ax.set_title(f'3D View of {col.replace("_", " ").title()} (Avg. of {run_count} runs)')
            
            fig.colorbar(surf, ax=ax, shrink=0.5, aspect=5)
            plt.tight_layout()
            
            plt.savefig(f'{output_dir}/3d_avg_{col}.png', dpi=300)
            print(f"Created 3D plot: {output_dir}/3d_avg_{col}.png")
    except Exception as e:
        print(f"Error creating 3D plots: {e}")

def plot_dbsize_data(df, output_dir):
    """Create plots for database size results"""
    # Sort by db_size for better visualization
    df = df.sort_values('db_size')
    
    # Plot 1: PIR Operation Times
    plt.figure(figsize=(12, 8))
    
    time_columns = [col for col in ['setup_time', 'query_time', 'answer_time', 'reconstruct_time'] if col in df.columns]
    markers = ['o', 's', '^', 'd']
    
    for i, col in enumerate(time_columns):
        marker = markers[i % len(markers)]
        plt.plot(df['db_size'], df[col], marker=marker, linewidth=2, label=col.replace('_', ' ').title())
    
    plt.xscale('log')
    plt.yscale('log')
    plt.xlabel('Database Size (entries)')
    plt.ylabel('Time (ms)')
    plt.title('PIR Operation Times vs Database Size')
    plt.legend()
    plt.tight_layout()
    
    plt.savefig(f'{output_dir}/dbsize_times.png', dpi=300)
    print(f"Created plot: {output_dir}/dbsize_times.png")
    
    # Plot 2: Stacked Bar Chart (if we have all needed columns)
    if all(col in df.columns for col in ['setup_time', 'query_time', 'answer_time', 'reconstruct_time']):
        plt.figure(figsize=(12, 8))
        operations = ['setup_time', 'query_time', 'answer_time', 'reconstruct_time']
        labels = ['Setup', 'Query Building', 'Query Answering', 'Reconstruction']
        colors = ['#1f77b4', '#ff7f0e', '#2ca02c', '#d62728']
        
        bottom = np.zeros(len(df))
        for i, col in enumerate(operations):
            plt.bar(df['db_size'], df[col], bottom=bottom, label=labels[i], color=colors[i])
            bottom += df[col].values
        
        plt.xscale('log')
        plt.xlabel('Database Size (entries)')
        plt.ylabel('Time (ms)')
        plt.title('PIR Operation Time Breakdown')
        plt.legend()
        plt.tight_layout()
        
        plt.savefig(f'{output_dir}/dbsize_time_breakdown.png', dpi=300)
        print(f"Created plot: {output_dir}/dbsize_time_breakdown.png")
    
    # Plot 3: Network usage (if available)
    network_cols = [col for col in ['offline_download', 'online_upload', 'online_download'] if col in df.columns]
    if network_cols:
        plt.figure(figsize=(12, 8))
        markers = ['o', 's', '^']
        
        for i, col in enumerate(network_cols):
            marker = markers[i % len(markers)]
            plt.plot(df['db_size'], df[col], marker=marker, linewidth=2, 
                     label=col.replace('_', ' ').title())
        
        plt.xscale('log')
        plt.yscale('log')
        plt.xlabel('Database Size (entries)')
        plt.ylabel('Data Transfer (KB)')
        plt.title('PIR Network Usage vs Database Size')
        plt.legend()
        plt.tight_layout()
        
        plt.savefig(f'{output_dir}/dbsize_network.png', dpi=300)
        print(f"Created plot: {output_dir}/dbsize_network.png")

def plot_recordsize_data(df, output_dir):
    """Create plots for record size results"""
    # Sort by record_size for better visualization
    df = df.sort_values('record_size')
    
    # Plot 1: PIR Operation Times
    plt.figure(figsize=(12, 8))
    
    time_columns = [col for col in ['setup_time', 'query_time', 'answer_time', 'reconstruct_time'] if col in df.columns]
    markers = ['o', 's', '^', 'd']
    
    for i, col in enumerate(time_columns):
        marker = markers[i % len(markers)]
        plt.plot(df['record_size'], df[col], marker=marker, linewidth=2, label=col.replace('_', ' ').title())
    
    plt.xscale('log', base=2)  # Record sizes are typically powers of 2
    plt.yscale('log')
    plt.xlabel('Record Size (bits)')
    plt.ylabel('Time (ms)')
    plt.title('PIR Operation Times vs Record Size')
    plt.legend()
    plt.tight_layout()
    
    plt.savefig(f'{output_dir}/recordsize_times.png', dpi=300)
    print(f"Created plot: {output_dir}/recordsize_times.png")
    
    # Similar charts as for dbsize (stacked bar, network usage) can be added here

def plot_combination_data(df, output_dir):
    """Create heatmaps for combination test results"""
    # Check if we have enough data for heatmaps
    if len(df['db_size'].unique()) < 3 or len(df['record_size'].unique()) < 3:
        print("Not enough unique values for heatmaps. Creating regular plots instead.")
        plot_generic_data(df, "db_recordsize", output_dir)
        return
        
    # Create heatmaps for each metric
    time_columns = [col for col in ['setup_time', 'query_time', 'answer_time', 'reconstruct_time'] if col in df.columns]
    
    for col in time_columns:
        try:
            pivot_df = df.pivot(index="db_size", columns="record_size", values=col)
            
            plt.figure(figsize=(12, 10))
            sns.heatmap(pivot_df, annot=True, fmt=".2f", cmap="viridis", 
                         cbar_kws={'label': 'Time (ms)'})
            
            plt.title(f'{col.replace("_", " ").title()} for Different DB and Record Sizes')
            plt.tight_layout()
            
            plt.savefig(f'{output_dir}/heatmap_{col}.png', dpi=300)
            print(f"Created heatmap: {output_dir}/heatmap_{col}.png")
        except Exception as e:
            print(f"Error creating heatmap for {col}: {e}")
    
    # 3D visualization if matplotlib has mplot3d
    try:
        from mpl_toolkits.mplot3d import Axes3D
        
        for col in time_columns:
            fig = plt.figure(figsize=(12, 10))
            ax = fig.add_subplot(111, projection='3d')
            
            x = df['db_size']
            y = df['record_size']
            z = df[col]
            
            surf = ax.plot_trisurf(np.log10(x), np.log2(y), z, cmap='viridis', 
                                 edgecolor='none', alpha=0.8)
            
            ax.set_xlabel('Database Size (log10)')
            ax.set_ylabel('Record Size (log2)')
            ax.set_zlabel(f'{col.replace("_", " ").title()} (ms)')
            ax.set_title(f'3D View of {col.replace("_", " ").title()}')
            
            fig.colorbar(surf, ax=ax, shrink=0.5, aspect=5)
            plt.tight_layout()
            
            plt.savefig(f'{output_dir}/3d_{col}.png', dpi=300)
            print(f"Created 3D plot: {output_dir}/3d_{col}.png")
    except Exception as e:
        print(f"Error creating 3D plots: {e}")

def plot_generic_data(df, plot_type, output_dir):
    """Create generic plots for any data"""
    print(f"Creating generic plots for {plot_type} data...")
    
    # Identify numeric columns
    numeric_cols = df.select_dtypes(include=['float64', 'int64']).columns.tolist()
    
    # Find likely x-axis (look for size, count, etc.)
    x_col = None
    for candidate in ['db_size', 'record_size', 'size', 'count']:
        if candidate in df.columns:
            x_col = candidate
            break
    
    if not x_col and numeric_cols:
        x_col = numeric_cols[0]  # Use the first numeric column if no size column found
    
    if not x_col:
        print("Error: No suitable x-axis column found in the data")
        return
    
    # Create plots for each numeric column vs x_col
    for col in numeric_cols:
        if col not in [x_col, 'run_count', 'run_id']:  # Skip certain columns
            plt.figure(figsize=(10, 6))
            plt.plot(df[x_col], df[col], marker='o', linewidth=2)
            
            plt.xlabel(x_col.replace('_', ' ').title())
            plt.ylabel(col.replace('_', ' ').title())
            
            # Add run count to title if available
            if 'run_count' in df.columns:
                run_count = int(df['run_count'].iloc[0])
                plt.title(f'{col.replace("_", " ").title()} vs {x_col.replace("_", " ").title()} (Avg. of {run_count} runs)')
            else:
                plt.title(f'{col.replace("_", " ").title()} vs {x_col.replace("_", " ").title()}')
                
            plt.grid(True)
            
            # Use log scale if values span multiple orders of magnitude
            if df[x_col].max() / max(df[x_col].min(), 1) > 100:
                plt.xscale('log')
            if df[col].max() / max(df[col].min(), 1) > 100:
                plt.yscale('log')
                
            plt.tight_layout()
            
            output_file = f'{output_dir}/{plot_type}_{col}_vs_{x_col}.png'
            plt.savefig(output_file, dpi=300)
            print(f"Created plot: {output_file}")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Generate plots from PIR test results")
    parser.add_argument("file", nargs="?", default=None, help="CSV file to plot (e.g., results/dbsize_results.csv)")
    parser.add_argument("--all", action="store_true", help="Plot all CSV files in the results directory")
    parser.add_argument("--output", default="plots", help="Directory to save plot images")
    parser.add_argument("--avg", action="store_true", help="Only plot files with average results (_avg)")
    
    args = parser.parse_args()
    
    if args.all:
        # Plot all CSV files in the results directory
        results_dir = "results"
        if not os.path.exists(results_dir):
            print(f"Error: Results directory '{results_dir}' not found")
            exit(1)
            
        csv_files = [os.path.join(results_dir, f) for f in os.listdir(results_dir) 
                    if f.endswith('_results.csv')]
        
        # Filter for average files if requested
        if args.avg:
            csv_files = [f for f in csv_files if '_avg_results.csv' in f]
        
        if not csv_files:
            print(f"No CSV files found in '{results_dir}'")
            exit(1)
            
        print(f"Found {len(csv_files)} CSV files to plot")
        for csv_file in csv_files:
            generate_plots(csv_file, args.output)
            
    elif args.file:
        # Plot a specific file
        generate_plots(args.file, args.output)
    else:
        # No arguments provided - try to plot common files
        common_files = []
        
        if args.avg:
            common_files = [
                "results/dbsize_avg_results.csv",
                "results/recordsize_avg_results.csv",
                "results/db_recordsize_avg_results.csv"
            ]
        else:
            common_files = [
                "results/dbsize_results.csv",
                "results/recordsize_results.csv",
                "results/db_recordsize_results.csv"
            ]
        
        found_files = False
        for file in common_files:
            if os.path.exists(file):
                found_files = True
                generate_plots(file, args.output)
                
        if not found_files:
            parser.print_help()