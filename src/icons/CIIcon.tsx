import { Link } from "@mui/material";

export function CIIcon(): JSX.Element {
  return (
    <Link href="https://github.com/aws/amazon-cloudwatch-agent/actions/workflows/integrationTest.yml">
      <img alt="CI Badge" src="https://github.com/aws/amazon-cloudwatch-agent/actions/workflows/integrationTest.yml/badge.svg" />
    </Link>
  );
}