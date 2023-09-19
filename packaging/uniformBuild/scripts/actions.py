import requests

OWNER ="aws"
REPO = "amazon-cloudwatch-agent"
GITHUB_API = f"https://api.github.com/repos/{OWNER}/{REPO}/actions"
id = "6202732096"
GITHUB_API_RUN = GITHUB_API + f"/runs/{id}"
GITHUB_API_JOB =  GITHUB_API_RUN + "/jobs?per_page=100"
data = requests.get(GITHUB_API_RUN).json()
print(data)
print(data['status'],data['conclusion'])
if data.get('conclusion') =='failure':
    jobs = []
    for page in range(3):
        jobdata = requests.get(GITHUB_API_JOB+f"&page={page}").json()
        jobs += jobdata.get('jobs')
    print(len(jobs))
    failed = []
    job_ids = []
    for job in jobs:
        id = job.get('id')
        if id in job_ids:
            continue
        # print(job.get("name"),job.get("conclusion"))
        if job.get("conclusion") == 'failure':
            failed.append(job.get("name"))
            # print(job.get("name"),"failed")
        job_ids.append(id)
    print(len(failed),"\n".join(failed))