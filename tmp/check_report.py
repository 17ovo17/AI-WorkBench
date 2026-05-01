import json, sys, re
d = json.load(open("D:/ai-workbench/tmp/v5final.json", encoding="utf-8"))
c = d.get("data", {}).get("content", "")
print("len:", len(c))
checks = {"健康评分": "健康评分" in c, "处置建议": "处置建议" in c, "历史对比": "历史对比" in c, "表格": "| 指标" in c or "| 主机" in c or "| 优先级" in c, "巡检报告": "巡检报告" in c}
for k, v in checks.items():
    status = "PASS" if v else "FAIL"
    print(k + ": " + status)
high = [v for v in re.findall(r"(\d+\.?\d*)%", c[:3000]) if float(v) > 100]
if high:
    print("FAIL CPU:", high)
else:
    print("PASS: all CPU <=100%")
print(c[:800])
