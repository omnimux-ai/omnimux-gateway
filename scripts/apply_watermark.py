#!/usr/bin/env python3
import json
import os
import subprocess
import sys

CONFIG_PATH = os.path.abspath(os.path.join(os.path.dirname(__file__), "../deliverables/stickers/watermark_config.json"))

def get_video_info(video_path):
    cmd = [
        "ffprobe", "-v", "error",
        "-select_streams", "v:0",
        "-show_entries", "stream=width,height",
        "-of", "json",
        video_path
    ]
    res = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    if res.returncode != 0:
        raise RuntimeError(f"无法读取视频信息: {res.stderr}")
    data = json.loads(res.stdout)
    w = int(data["streams"][0]["width"])
    h = int(data["streams"][0]["height"])
    return w, h

def apply_watermark(input_video, output_video=None):
    if not os.path.exists(CONFIG_PATH):
        raise FileNotFoundError(f"配置文件不存在: {CONFIG_PATH}")
        
    with open(CONFIG_PATH, "r", encoding="utf-8") as f:
        config = json.load(f)
        
    badge_file = config["badge_file"]
    if not os.path.exists(badge_file):
        raise FileNotFoundError(f"水印贴纸不存在: {badge_file}")
        
    if not os.path.exists(input_video):
        raise FileNotFoundError(f"输入视频不存在: {input_video}")
        
    if output_video is None:
        base, ext = os.path.splitext(input_video)
        output_video = f"{base}_watermarked{ext}"

    rules = config["placement"]["rules"]
    vw, vh = get_video_info(input_video)
    
    # 根据配置比例自适应计算贴纸宽度与边距
    target_wm_width = int(vw * rules["scale_width_ratio"])
    margin_x = int(vw * rules["margin_left_ratio"])
    margin_y = int(vh * rules["margin_top_ratio"])
    
    filter_expr = f"[1:v]scale={target_wm_width}:-1[wm];[0:v][wm]overlay=x={margin_x}:y={margin_y}"
    
    cmd = [
        "ffmpeg", "-y",
        "-i", input_video,
        "-i", badge_file,
        "-filter_complex", filter_expr,
        "-c:v", config["render_engine"].get("video_codec", "libx264"),
        "-preset", config["render_engine"].get("preset", "fast"),
        "-crf", str(config["render_engine"].get("crf", 19)),
        "-c:a", config["render_engine"].get("audio_codec", "copy"),
        output_video
    ]
    
    print(f"正在给视频添加水印: {input_video}")
    print(f"视频分辨率: {vw}x{vh} | 水印宽度: {target_wm_width}px | 位置: (X={margin_x}, Y={margin_y})")
    
    subprocess.run(cmd, check=True)
    print(f"加水印成功，输出路径: {output_video}")
    return output_video

if __name__ == "__main__":
    if len(sys.argv) < 2 or sys.argv[1] in ("-h", "--help"):
        print("用法: python3 scripts/apply_watermark.py <input_video> [output_video]")
        sys.exit(0)
    inp = sys.argv[1]
    out = sys.argv[2] if len(sys.argv) > 2 else None
    apply_watermark(inp, out)
