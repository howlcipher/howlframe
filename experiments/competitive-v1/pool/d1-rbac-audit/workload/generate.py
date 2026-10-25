#!/usr/bin/env python3
import json
import os
import random

def main():
    random.seed(8421)
    dir_path = os.path.dirname(os.path.abspath(__file__))

    boundaries = {
        "max_session_duration_hours": 12,
        "forbidden_actions": ["iam:DeleteAccount", "billing:*", "root:*"],
        "privilege_escalation_pairs": [
            ["iam:PassRole", "lambda:CreateFunction"],
            ["iam:CreateAccessKey", "iam:UpdateAssumeRolePolicy"],
            ["ec2:RunInstances", "iam:PassRole"]
        ]
    }
    with open(os.path.join(dir_path, "boundaries.json"), "w") as f:
        json.dump(boundaries, f, indent=2)

    users = ["alice", "bob", "carol", "dave", "eve", "frank", "grace"]
    actions_pool = [
        "s3:GetObject", "s3:PutObject", "s3:ListBucket",
        "ec2:DescribeInstances", "ec2:StartInstances", "ec2:StopInstances",
        "lambda:InvokeFunction", "sqs:SendMessage", "sns:Publish",
        "iam:PassRole", "lambda:CreateFunction", "billing:ViewBill", "iam:DeleteAccount"
    ]

    roles = []
    for i in range(1, 151):
        r_name = f"Role-{i:03d}"
        assigned = random.sample(users, k=random.randint(1, 4))
        dur = random.choice([4, 8, 12, 16])
        perms = random.sample(actions_pool, k=random.randint(2, 6))
        roles.append({
            "role_name": r_name,
            "assigned_users": assigned,
            "session_duration_hours": dur,
            "permissions": perms
        })

    with open(os.path.join(dir_path, "iam.json"), "w") as f:
        json.dump({"roles": roles}, f, indent=2)

if __name__ == "__main__":
    main()
